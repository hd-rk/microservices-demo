import os
import requests
import redis
import json

APIKEY = os.environ.get('APIKEY')
DEFAULT_CURRENCY = os.environ.get('DEFAULT_CURRENCY')
CURRENCY_SUPPORT = os.environ.get('CURRENCY_SUPPORT').split(',')

# r = redis.Redis(host='localhost', port=6379, db=0)
r = redis.Redis(host=os.environ.get('REDIS_HOST'), port=int(os.environ.get('REDIS_PORT')), db=0)

payload = {DEFAULT_CURRENCY:1}

for currency in CURRENCY_SUPPORT:
    response = requests.get(f'https://www.alphavantage.co/query?function=CURRENCY_EXCHANGE_RATE&from_currency={DEFAULT_CURRENCY}&to_currency={currency}&apikey=DEO388ZM3UEZ34M8')
    response_dict = response.json()
    first_key = list(response_dict)[0]
    fifth_key = list(response_dict[first_key])[4]
    payload[currency] = response_dict[first_key][fifth_key]
    # r.set(DEFAULT_CURRENCY+'_'+currency, response_dict[first_key][fifth_key])
    
    # response = requests.get(f'https://www.alphavantage.co/query?function=CURRENCY_EXCHANGE_RATE&from_currency={currency}&to_currency={DEFAULT_CURRENCY}&apikey=DEO388ZM3UEZ34M8')
    # response_dict = response.json()
    # first_key = list(response_dict)[0]
    # fifth_key = list(response_dict[first_key])[4]
    # print(currency+'_'+DEFAULT_CURRENCY+': '+response_dict[first_key][fifth_key])
    # r.set(currency+'_'+DEFAULT_CURRENCY, response_dict[first_key][fifth_key])
r.set('currency_conversion', json.dumps(payload))