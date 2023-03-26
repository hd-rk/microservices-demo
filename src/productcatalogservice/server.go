// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"cloud.google.com/go/profiler"
	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	cat          pb.ListProductsResponse
	catalogMutex *sync.Mutex
	log          *logrus.Logger
	extraLatency time.Duration

	port = "3550"

	reloadCatalog bool
)

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	catalogMutex = &sync.Mutex{}
	err := readCatalogFile(&cat)
	if err != nil {
		log.Warnf("could not parse product catalog")
	}
}

func connectDb(ctx context.Context, serverUri, serverDb string) (*mongo.Collection, error) {

	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(serverUri).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(ctx, opts)

	if err != nil {
		log.Errorf("Failed to connect to MongoDB %v", err)
		return nil, err
	}

	collection := client.Database(serverDb).Collection("products")
	return collection, nil
}

type productCatalog struct {
	pb.UnimplementedProductCatalogServiceServer
	productColl *mongo.Collection
}

type MongoProduct struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name,omitempty"`
	Description string             `bson:"description,omitempty"`
	Picture     string             `bson:"picture,omitempty"`
	PriceUnits  int64              `bson:"price_units,omitempty"`
	PriceNanos  int32              `bson:"price_nanos,omitempty"`
	Categories  []string           `bson:"categories,omitempty"`
	Units       int32              `bson:"units,omitempty"`
	Sold        int32              `bson:"sold,omitempty"`
}

func main() {
	if os.Getenv("ENABLE_TRACING") == "1" {
		err := initTracing()
		if err != nil {
			log.Warnf("warn: failed to start tracer: %+v", err)
		}
	} else {
		log.Info("Tracing disabled.")
	}

	if os.Getenv("DISABLE_PROFILER") == "" {
		log.Info("Profiling enabled.")
		go initProfiling("productcatalogservice", "1.0.0")
	} else {
		log.Info("Profiling disabled.")
	}

	flag.Parse()

	// set injected latency
	if s := os.Getenv("EXTRA_LATENCY"); s != "" {
		v, err := time.ParseDuration(s)
		if err != nil {
			log.Fatalf("failed to parse EXTRA_LATENCY (%s) as time.Duration: %+v", v, err)
		}
		extraLatency = v
		log.Infof("extra latency enabled (duration: %v)", extraLatency)
	} else {
		extraLatency = time.Duration(0)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for {
			sig := <-sigs
			log.Printf("Received signal: %s", sig)
			if sig == syscall.SIGUSR1 {
				reloadCatalog = true
				log.Infof("Enable catalog reloading")
			} else {
				reloadCatalog = false
				log.Infof("Disable catalog reloading")
			}
		}
	}()

	// mongodb+srv://demo-user:geer8nqi3AVf@demo-mongodb-svc.mongodb.svc.cluster.local/product_catalog?replicaSet=demo-mongodb&ssl=false&authSource=admin
	mongoUri := fmt.Sprintf(
		"%s://%s:%s@%s/%s?replicaSet=%s&ssl=%s&authSource=admin",
		getEnv("MONGODB_PROTOCOL", "mongodb+srv"),
		getEnv("MONGODB_USERNAME", "username"),
		getEnv("MONGODB_PASSWORD", "password"),
		getEnv("MONGODB_HOST", "localhost"),
		getEnv("MONGODB_DATABASE", "product_catalog"),
		getEnv("MONGODB_RS", "demo-mongodb"),
		getEnv("MONGODB_SSL", "false"),
	)

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	log.Infof("starting grpc server at :%s", port)
	run(port, mongoUri, getEnv("MONGODB_DATABASE", "product_catalog"))
	select {}
}

func run(port, mongoDBUri, mongoDatabase string) string {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	// Propagate trace context
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}))
	var srv *grpc.Server
	srv = grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection, err := connectDb(ctx, mongoDBUri, mongoDatabase)
	svc := &productCatalog{
		productColl: collection,
	}

	svc.loadProductIntoMongo()

	pb.RegisterProductCatalogServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	go srv.Serve(l)
	return l.Addr().String()
}

func initStats() {
	// TODO(drewbr) Implement OpenTelemetry stats
}

func initTracing() error {
	var (
		collectorAddr string
		collectorConn *grpc.ClientConn
	)

	ctx := context.Background()

	mustMapEnv(&collectorAddr, "COLLECTOR_SERVICE_ADDR")
	mustConnGRPC(ctx, &collectorConn, collectorAddr)

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithGRPCConn(collectorConn))
	if err != nil {
		log.Warnf("warn: Failed to create trace exporter: %v", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	return err
}

func initProfiling(service, version string) {
	// TODO(ahmetb) this method is duplicated in other microservices using Go
	// since they are not sharing packages.
	for i := 1; i <= 3; i++ {
		if err := profiler.Start(profiler.Config{
			Service:        service,
			ServiceVersion: version,
			// ProjectID must be set if not running on GCP.
			// ProjectID: "my-project",
		}); err != nil {
			log.Warnf("failed to start profiler: %+v", err)
		} else {
			log.Info("started Stackdriver profiler")
			return
		}
		d := time.Second * 10 * time.Duration(i)
		log.Infof("sleeping %v to retry initializing Stackdriver profiler", d)
		time.Sleep(d)
	}
	log.Warn("could not initialize Stackdriver profiler after retrying, giving up")
}

func readCatalogFile(catalog *pb.ListProductsResponse) error {
	catalogMutex.Lock()
	defer catalogMutex.Unlock()
	catalogJSON, err := ioutil.ReadFile("products.json")
	if err != nil {
		log.Fatalf("failed to open product catalog json file: %v", err)
		return err
	}
	if err := jsonpb.Unmarshal(bytes.NewReader(catalogJSON), catalog); err != nil {
		log.Warnf("failed to parse the catalog JSON: %v", err)
		return err
	}
	log.Info("successfully parsed product catalog json")
	return nil
}

func parseCatalog() []*pb.Product {
	if reloadCatalog || len(cat.Products) == 0 {
		err := readCatalogFile(&cat)
		if err != nil {
			return []*pb.Product{}
		}
	}
	return cat.Products
}

func productMongoToGPB(mp *MongoProduct) *pb.Product {
	return &pb.Product{
		Id:          mp.ID.Hex(),
		Name:        mp.Name,
		Description: mp.Description,
		Picture:     mp.Picture,
		PriceUsd: &pb.Money{
			Units:        mp.PriceUnits,
			Nanos:        mp.PriceNanos,
			CurrencyCode: "USD",
		},
		Categories: mp.Categories,
		Units:      mp.Units,
		Sold:       mp.Sold,
	}
}

func (p *productCatalog) loadProductIntoMongo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if count, err := p.productColl.CountDocuments(ctx, bson.D{}); err == nil && count > 0 {
		log.Infof("%d products in db, no loading needed.", count)
		return nil
	}
	products := parseCatalog()
	var docs []interface{}
	for _, product := range products {
		docs = append(docs, MongoProduct{
			Name:        product.Name,
			Description: product.Description,
			Picture:     product.Picture,
			PriceUnits:  product.PriceUsd.Units,
			PriceNanos:  product.PriceUsd.Nanos,
			Categories:  product.Categories,
			Units:       product.Units,
			Sold:        product.Sold,
		})
	}
	if result, err := p.productColl.InsertMany(context.TODO(), docs); err != nil {
		log.Warnf("Error loading product documents into DB %v", err)
		return err
	} else {
		log.Infof("%d inserted to db", len(result.InsertedIDs))
	}

	models := []mongo.IndexModel{
		Keys: bsonx.Doc{
			{Key: "name", Value: bsonx.String("text")},
			{Key: "description", Value: bsonx.String("text")},
		},
		Keys: bsonx.Doc{
			{Key: "sold", Value: bsonx.Int32(-1)},
		},
	}
	_, err := p.productColl.Indexes().CreateMany(ctx, models)

	if err != nil {
		return err
	}

	return nil
}

func (p *productCatalog) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (p *productCatalog) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (p *productCatalog) ListProducts(ctx context.Context, req *pb.Empty) (*pb.ListProductsResponse, error) {
	time.Sleep(extraLatency)
	var ps []*pb.Product
	var products []MongoProduct
	cursor, err := p.productColl.Find(ctx, bson.D{})
	if err != nil {
		log.Errorf("No documents regarding product catalog were found.")
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &products); err != nil {
		log.Errorf("Couldn't retrieve the products")
		return nil, err
	}
	// return &pb.ListProductsResponse{Products: parseCatalog()}, nil
	for _, mp := range products {
		ps = append(ps, productMongoToGPB(&mp))
	}
	return &pb.ListProductsResponse{Products: ps}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	time.Sleep(extraLatency)
	var mp MongoProduct
	objectId, err := primitive.ObjectIDFromHex(req.GetId())
	if err != nil {
		log.Errorf("Invalid product ID %s", req.GetId())
		return nil, err
	}
	if err = p.productColl.FindOne(ctx, bson.M{"_id": objectId}).Decode(&mp); err != nil {
		log.Errorf("Error retrieving product %v", err)
		return nil, err
	}
	if &mp == nil {
		return nil, status.Errorf(codes.NotFound, "no product with ID %s", req.Id)
	}
	return productMongoToGPB(&mp), nil
}

func (p *productCatalog) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	time.Sleep(extraLatency)

	// Intepret query as a substring match in name or description.
	var ps []*pb.Product

	filter := bson.D{{"$text", bson.D{{"$search", req.Query}}}}
	cursor, err := p.productColl.Find(ctx, filter)
	if err != nil {
		log.Errorf("No documents regarding product catalog were found.")
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var mp MongoProduct
		if err := cursor.Decode(&mp); err != nil {
			log.Errorf("err retrieving product %v", err)
		}
		ps = append(ps, productMongoToGPB(&mp))
	}

	return &pb.SearchProductsResponse{Results: ps}, nil
}

func (p *productCatalog) GetRecommendations(ctx context.Context, req *pb.Empty) (*pb.SearchProductsResponse, error) {
	time.Sleep(extraLatency)
	// Intepret query as a substring match in name or description.
	var ps []*pb.Product
	// for _, p := range parseCatalog() {
	//  if strings.Contains(strings.ToLower(p.Name), strings.ToLower(req.Query)) ||
	//    strings.Contains(strings.ToLower(p.Description), strings.ToLower(req.Query)) && p.UnitsBought {
	//    ps = append(ps, p)
	//  }
	// }

	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{"sold", -1}}).SetLimit(5)
	cursor, err := p.productColl.Find(ctx, filter, opts)
	if err != nil {
		log.Errorf("No documents regarding product catalog were found.")
		return nil, err
	}

	if err = cursor.All(ctx, ps); err != nil {
		log.Errorf("Couldn't retrieve the products")
		return nil, err
	}

	return &pb.SearchProductsResponse{Results: ps}, nil
}

func (p *productCatalog) UpdateProductCount(ctx context.Context, req *pb.UpdateProductCountRequest) (*pb.UpdateProductCountResponse, error) {
	time.Sleep(extraLatency)

	var found *MongoProduct
	objectId, err := primitive.ObjectIDFromHex(req.GetId())
	var response *pb.UpdateProductCountResponse
	response.Id = objectId
	filter := bson.D{{"_id", objectId}}

	err := p.productColl.FindOne(ctx, filter).Decode(found)
	if err != nil {
		log.Warnf("Query exception %s", err)
		return nil, status.Errorf(codes.Internal, "Query exception")
	} else if found == nil {
		return nil, status.Errorf(codes.NotFound, "no product with ID %s", req.Id)
	}
	updatedCount := found.Units - req.GetCount()
	if updatedCount < 0 {
		return response, errors.New("update fail: Count is more than number of available units.")
	}

	document := bson.D{
		{"name", found.Name},
		{"description", found.Description},
		{"picture", found.Picture},
		{"price_usd", found.PriceUsd},
		{"categories", found.Categories},
		{"units", found.Units - req.GetCount()},
		{"sold", found.Sold + req.GetCount()},
	}
	update := bson.D{{"$set", document}}
	_, err = p.productColl.UpdateOne(ctx, filter, update)

	return response, nil
}

func mustMapEnv(target *string, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("environment variable %q not set", envKey))
	}
	*target = v
}

func mustConnGRPC(ctx context.Context, conn **grpc.ClientConn, addr string) {
	var err error
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	*conn, err = grpc.DialContext(ctx, addr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	if err != nil {
		panic(errors.Wrapf(err, "grpc: failed to connect %s", addr))
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
