#!/usr/bin/python
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
import random
import time
import traceback
from concurrent import futures

import googlecloudprofiler
from google.auth.exceptions import DefaultCredentialsError
import grpc

import demo_pb2
import demo_pb2_grpc
from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc

from opentelemetry import trace
from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient, GrpcInstrumentorServer
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
# import redis
import time
import pymongo
# import threading

from logger import getJSONLogger
logger = getJSONLogger('recommendationservice-server')


def initStackdriverProfiling():
    project_id = None
    try:
        project_id = os.environ["GCP_PROJECT_ID"]
    except KeyError:
        # Environment variable not set
        pass

    for retry in range(1, 4):
        try:
            if project_id:
                googlecloudprofiler.start(service='recommendation_server', service_version='1.0.0', verbose=0, project_id=project_id)
            else:
                googlecloudprofiler.start(service='recommendation_server', service_version='1.0.0', verbose=0)
            logger.info("Successfully started Stackdriver Profiler.")
            return
        except (BaseException) as exc:
            logger.info("Unable to start Stackdriver Profiler Python agent. " + str(exc))
            if (retry < 4):
                logger.info("Sleeping %d seconds to retry Stackdriver Profiler agent initialization"%(retry*10))
                time.sleep(1)
            else:
                logger.warning("Could not initialize Stackdriver Profiler after retrying, giving up")
    return


class RecommendationService(demo_pb2_grpc.RecommendationServiceServicer):
    def __init__(self):
        mongo_uri = os.environ.get('MONGO_CONNECTION_URL', '')
        self.client = pymongo.MongoClient(mongo_uri)
        self.db = self.client['recommendation']
        self.max_responses = 5
        self.max_cached_products = 20
        # self.cache_expiry = 300  # 5 minutes in seconds
        # self.last_cache_update = 0
        # start background thread to update cache periodically
        # self.cache_thread = threading.Thread(target=self.update_cache)
        # self.cache_thread.daemon = True
        # self.cache_thread.start()
        # docker run --network=mongonet -e MONGO_CONNECTION_URL=mongodb://my-mongo:27017 -e PRODUCT_CATALOG_SERVICE_ADDR=34.69.33.183:3550 -p 8080:8080 recommendation

    def update_cache(self):
        # fetch list of products from product catalog stub
        catalog_addr = os.environ.get('PRODUCT_CATALOG_SERVICE_ADDR', '')
        if catalog_addr == "":
            raise Exception('PRODUCT_CATALOG_SERVICE_ADDR environment variable not set')
        logger.info("product catalog address: " + catalog_addr)
        if catalog_addr.endswith(":443"):
            channel = grpc.secure_channel(catalog_addr, grpc.ssl_channel_credentials())
        else:
            channel = grpc.insecure_channel(catalog_addr)
        product_catalog_stub = demo_pb2_grpc.ProductCatalogServiceStub(channel)
        cat_response = product_catalog_stub.GetRecommendations(demo_pb2.Empty())
        top_products = [x.id for x in cat_response.results]
        # top_products = product_ids[:self.max_cached_products]
        logger.info(f"top_products = {top_products}")
        
        self.db.cache.update_one(
            {'_id': 'top_products'},
            {'$set': {'products': top_products}},
            upsert=True
        )
        # self.last_cache_update = time.time()
        # time.sleep(self.cache_expiry)

    def ListRecommendations(self, request, context):
        # retrieve top products from cache
        cache = self.db.cache.find_one({'_id': 'top_products'})
        if not cache:
            self.update_cache()
            cache = self.db.cache.find_one({'_id': 'top_products'})
        top_products = cache['products']

        # filter and sample products
        filtered_products = list(set(top_products)-set(request.product_ids))
        num_products = len(filtered_products)
        num_return = min(self.max_responses, num_products)
        indices = random.sample(range(num_products), num_return)
        prod_list = [filtered_products[i] for i in indices]
        logger.info("[Recv ListRecommendations] product_ids={}".format(prod_list))

        # build and return response
        response = demo_pb2.ListRecommendationsResponse()
        response.product_ids.extend(top_products)
        return response

    # def ListRecommendations(self, request, context):
    #     max_responses = 5
    #     # fetch list of products from product catalog stub
    #     cat_response = product_catalog_stub.ListProducts(demo_pb2.Empty())
    #     product_ids = [x.id for x in cat_response.products]
    #     filtered_products = list(set(product_ids)-set(request.product_ids))
    #     num_products = len(filtered_products)
    #     num_return = min(max_responses, num_products)
    #     # sample list of indicies to return
    #     indices = random.sample(range(num_products), num_return)
    #     # fetch product ids from indices
    #     prod_list = [filtered_products[i] for i in indices]
    #     logger.info("[Recv ListRecommendations] product_ids={}".format(prod_list))
    #     # build and return response
    #     response = demo_pb2.ListRecommendationsResponse()
    #     response.product_ids.extend(prod_list)
    #     return response

    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING)

    def Watch(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.UNIMPLEMENTED)


if __name__ == "__main__":
    logger.info("initializing recommendationservice")

    service = RecommendationService()
    if "ROLE" in os.environ and os.environ["ROLE"] == "updater":
        try:
            logger.info("starting recommendation cache update")
            service.update_cache()
            logger.info("completed recommendation cache update")
        except Exception as err:
            print(err)
            logger.warning(err)
    else:
        try:
            if "DISABLE_PROFILER" in os.environ:
                raise KeyError()
            else:
                logger.info("Profiler enabled.")
                initStackdriverProfiling()
        except KeyError:
            logger.info("Profiler disabled.")

        try:
            grpc_client_instrumentor = GrpcInstrumentorClient()
            grpc_client_instrumentor.instrument()
            grpc_server_instrumentor = GrpcInstrumentorServer()
            grpc_server_instrumentor.instrument()
            if os.environ["ENABLE_TRACING"] == "1":
                trace.set_tracer_provider(TracerProvider())
                otel_endpoint = os.getenv("COLLECTOR_SERVICE_ADDR", "localhost:4317")
                trace.get_tracer_provider().add_span_processor(
                    BatchSpanProcessor(
                        OTLPSpanExporter(
                            endpoint=otel_endpoint,
                            insecure=True)))
        except (KeyError, DefaultCredentialsError):
            logger.info("Tracing disabled.")
        except Exception as e:
            logger.warning(f"Exception on Cloud Trace setup: {traceback.format_exc()}, tracing disabled.")
        port = os.environ.get('PORT', "8080")

        # create gRPC server
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

        # add class to gRPC server
        demo_pb2_grpc.add_RecommendationServiceServicer_to_server(service, server)
        health_pb2_grpc.add_HealthServicer_to_server(service, server)

        # start server
        logger.info("listening on port: " + port)
        server.add_insecure_port('[::]:'+port)
        server.start()

        # keep alive
        try:
            while True:
                time.sleep(10000)
        except KeyboardInterrupt:
            server.stop(0)
