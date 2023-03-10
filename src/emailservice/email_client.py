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

import grpc
import os
import demo_pb2
import demo_pb2_grpc

from logger import getJSONLogger
logger = getJSONLogger('emailservice-client')

def send_confirmation_email(email, order):
  channel = grpc.insecure_channel('{}:{}'.format(os.environ.get("EMAIL_SERVER_ADDR", "localhost"), os.environ.get("PORT","8080")))
  stub = demo_pb2_grpc.EmailServiceStub(channel)
  try:
    response = stub.SendOrderConfirmation(demo_pb2.SendOrderConfirmationRequest(
      email = email,
      order = order
    ))
    logger.info('Request sent.')
  except grpc.RpcError as err:
    logger.error(err.details())
    logger.error('{}, {}'.format(err.code().name, err.code().value))


if __name__ == '__main__':

  email = "JohnDoe@example.com"
  order_result = demo_pb2.OrderResult(
      order_id='12345',
      shipping_tracking_id='67890',
      shipping_cost=demo_pb2.Money(currency_code='USD', units=100, nanos=500000000),
      shipping_address=demo_pb2.Address(
          street_address='123 Main St',
          city='Anytown',
          state='CA',
          country='USA',
          zip_code=12345
      ),
      items=[
          demo_pb2.OrderItem(
              item=demo_pb2.CartItem(
                  product_id='ABC123',
                  quantity=2
              ),
              cost=demo_pb2.Money(currency_code='USD', units=50, nanos=0)
          ),
          demo_pb2.OrderItem(
              item=demo_pb2.CartItem(
                  product_id='DEF456',
                  quantity=1
              ),
              cost=demo_pb2.Money(currency_code='USD', units=25, nanos=0)
          )
      ]
  )

  send_confirmation_email(email='john@example.com',
      order=order_result)
