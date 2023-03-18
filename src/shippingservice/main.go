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
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"cloud.google.com/go/profiler"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/shippingservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultPort = "50051"
)

var log *logrus.Logger

type ShippingStatus int

const (
	ShippingOpen ShippingStatus = iota
	ShippingScheduled
	ShippingInProgress
	ShippingCompleted
)

func (ss ShippingStatus) String() string {
	switch ss {
	case 0:
		return "Open"
	case 1:
		return "Scheduled"
	case 2:
		return "In Progress"
	case 3:
		return "Completed"
	default:
		return "Unknown"
	}
}

type ShippingAddress struct {
	StreetAddress string
	City          string
	State         string
	Country       string
	ZipCode       int32
}

type ShippingOrder struct {
	gorm.Model
	ID      string
	Address ShippingAddress `gorm:"embedded;embeddedPrefix:address_"`
	Status  ShippingStatus
}

// server controls RPC service responses.
type server struct {
	pb.UnimplementedShippingServiceServer
	db *gorm.DB
}

func init() {
	log = logrus.New()
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
}

func main() {
	if os.Getenv("DISABLE_TRACING") == "" {
		log.Info("Tracing enabled, but temporarily unavailable")
		log.Info("See https://github.com/GoogleCloudPlatform/microservices-demo/issues/422 for more info.")
		go initTracing()
	} else {
		log.Info("Tracing disabled.")
	}

	if os.Getenv("DISABLE_PROFILER") == "" {
		log.Info("Profiling enabled.")
		go initProfiling("shippingservice", "1.0.0")
	} else {
		log.Info("Profiling disabled.")
	}

	var (
		dbHost     string
		dbPort     string
		dbUsername string
		dbPassword string
		dbName     string
		ok         bool
	)
	if dbHost, ok = os.LookupEnv("DB_HOST"); !ok {
		log.Fatal("Missing DB_HOST")
	}
	if dbPort, ok = os.LookupEnv("DB_PORT"); !ok {
		log.Fatal("Missing DB_PORT")
	}
	if dbUsername, ok = os.LookupEnv("DB_USERNAME"); !ok {
		log.Fatal("Missing DB_USERNAME")
	}
	if dbPassword, ok = os.LookupEnv("DB_PASSWORD"); !ok {
		log.Fatal("Missing DB_PASSWORD")
	}
	if dbName, ok = os.LookupEnv("DB_NAME"); !ok {
		log.Fatal("Missing DB_NAME")
	}

	dbUrl := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUsername,
		dbPassword,
		dbHost,
		dbPort,
		dbName)

	port := defaultPort
	if value, ok := os.LookupEnv("PORT"); ok {
		port = value
	}
	port = fmt.Sprintf(":%s", port)

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var srv *grpc.Server
	if os.Getenv("DISABLE_STATS") == "" {
		log.Info("Stats enabled, but temporarily unavailable")
		srv = grpc.NewServer()
	} else {
		log.Info("Stats disabled.")
		srv = grpc.NewServer()
	}

	db, err := gorm.Open(mysql.Open(dbUrl), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database @ %s", dbUrl)
	}
	db.AutoMigrate(&ShippingOrder{})

	svc := &server{
		db: db,
	}

	pb.RegisterShippingServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	log.Infof("Shipping Service listening on port %s", port)

	// Register reflection service on gRPC server.
	reflection.Register(srv)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Check is for health checking.
func (s *server) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (s *server) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

// GetQuote produces a shipping quote (cost) in USD.
func (s *server) GetQuote(ctx context.Context, in *pb.GetQuoteRequest) (*pb.GetQuoteResponse, error) {
	log.Info("[GetQuote] received request")
	defer log.Info("[GetQuote] completed request")

	// 1. Generate a quote based on the total number of items to be shipped.
	quote := CreateQuoteFromCount(0)

	// 2. Generate a response.
	return &pb.GetQuoteResponse{
		CostUsd: &pb.Money{
			CurrencyCode: "USD",
			Units:        int64(quote.Dollars),
			Nanos:        int32(quote.Cents * 10000000)},
	}, nil

}

// ShipOrder mocks that the requested items will be shipped.
// It supplies a tracking ID for notional lookup of shipment delivery status.
func (s *server) ShipOrder(ctx context.Context, in *pb.ShipOrderRequest) (*pb.ShipOrderResponse, error) {
	log.Info("[ShipOrder] received request")
	defer log.Info("[ShipOrder] completed request")
	// 1. Create a Tracking ID
	baseAddress := fmt.Sprintf(
		"%s, %s, %s",
		in.Address.StreetAddress,
		in.Address.City,
		in.Address.State)
	id := CreateTrackingId(baseAddress)

	s.db.Create(&ShippingOrder{
		ID:     id,
		Status: ShippingOpen,
		Address: ShippingAddress{
			StreetAddress: in.Address.StreetAddress,
			City:          in.Address.City,
			State:         in.Address.State,
			Country:       in.Address.Country,
			ZipCode:       in.Address.ZipCode,
		},
	})

	// 2. Generate a response.
	return &pb.ShipOrderResponse{
		TrackingId: id,
	}, nil
}

func (s *server) TrackOrder(ctx context.Context, in *pb.TrackShipOrderRequest) (*pb.TrackShipOrderResponse, error) {
	shippingOrder := ShippingOrder{
		ID: in.TrackingId,
	}
	res := s.db.First(&shippingOrder)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("Tracking ID %s not found", in.TrackingId)
	}
	return &pb.TrackShipOrderResponse{
		TrackingId: shippingOrder.ID,
		Status:     shippingOrder.Status.String(),
		Address: &pb.Address{
			StreetAddress: shippingOrder.Address.StreetAddress,
			City:          shippingOrder.Address.City,
			State:         shippingOrder.Address.State,
			Country:       shippingOrder.Address.Country,
			ZipCode:       shippingOrder.Address.ZipCode,
		},
	}, nil
}

func (s *server) UpdateOrderShippingStatus(ctx context.Context, in *pb.UpdateShipOrderStatusRequest) (*pb.UpdateShipOrderStatusResponse, error) {

	if in.Status < 0 || in.Status > 3 {
		return nil, fmt.Errorf("Tracking status %d not valid", in.Status)
	}

	shippingOrder := ShippingOrder{
		ID: in.TrackingId,
	}
	res := s.db.First(&shippingOrder)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("Tracking ID %s not found", in.TrackingId)
	}

	shippingOrder.Status = ShippingStatus(in.Status)
	s.db.Save(shippingOrder)

	return &pb.UpdateShipOrderStatusResponse{
		Success: true,
	}, nil
}

func initStats() {
	//TODO(arbrown) Implement OpenTelemetry stats
}

func initTracing() {
	// TODO(arbrown) Implement OpenTelemetry tracing
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
