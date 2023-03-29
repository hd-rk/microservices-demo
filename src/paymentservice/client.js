/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
require('@google-cloud/trace-agent').start();

const path = require('path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader')
const pino = require('pino');

const PROTO_PATH = path.join(__dirname, './proto/demo.proto');
const PORT = 7000;

var packageDefinition = protoLoader.loadSync(
  PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
    // includeDirs: [__dirname + '/proto/protobuf', __dirname + '/proto/googleapis'],
  });

const shopProto = grpc.loadPackageDefinition(packageDefinition).hipstershop;
const client = new shopProto.PaymentService(`localhost:${PORT}`,
  grpc.credentials.createInsecure());

const logger = pino({
  name: 'paymentservice-client',
  messageKey: 'message',
  formatters: {
    level (logLevelString, logLevelNum) {
      return { severity: logLevelString }
    }
  }
});

const request = {
  amount: {
    currency_code: 'USD',
    units: 123,
    nanos: 0
  },
  credit_card: {
    credit_card_number: '5555555555554444',
    credit_card_expiration_year: '2023',
    credit_card_expiration_month: '12'
  }
};


client.charge(request, (err, response) => {
  if (err) {
    console.error(`Error in convert: ${err}`);
  } else {
    console.log(`Response: ${JSON.stringify(response)}`);
  }
});
