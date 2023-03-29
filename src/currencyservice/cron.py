import os
import requests
import mysql.connector
import json
import traceback

APIKEY = os.environ.get('APIKEY')
DEFAULT_CURRENCY = os.environ.get('DEFAULT_CURRENCY')
CURRENCY_SUPPORT = os.environ.get('CURRENCY_SUPPORT').split(',')

cnx = mysql.connector.connect(
    user=os.environ.get('MYSQL_USER'),
    password=os.environ.get('MYSQL_PASSWORD'),
    host=os.environ.get('MYSQL_HOST'),
    database=os.environ.get('MYSQL_DATABASE')
)
cursor = cnx.cursor()

payload = {
    DEFAULT_CURRENCY: 1
}

# Create an insert query that adds the data to a table in the database
table_name = 'currency_conversion'
columns = 'currency_from, currency_to, exchange_rate'
values = '"'+DEFAULT_CURRENCY+'","'+DEFAULT_CURRENCY+'",1'
insert_query = f"INSERT INTO {table_name} ({columns}) VALUES ({values}) ON DUPLICATE KEY UPDATE exchange_rate=1"
cursor.execute(insert_query)
for currency in CURRENCY_SUPPORT:
    try:
        response = requests.get(
            url="https://www.alphavantage.co/query",
            params={
                "function": "CURRENCY_EXCHANGE_RATE",
                "from_currency": DEFAULT_CURRENCY,
                "to_currency": currency,
                "apikey": APIKEY
            })
        response_dict = response.json()
        first_key = list(response_dict)[0]
        fifth_key = list(response_dict[first_key])[4]
        exchange_rate = response_dict[first_key][fifth_key]
        print(f"{DEFAULT_CURRENCY}=>{currency} : {exchange_rate}")

        # Create an insert query that adds the data to a table in the database
        columns = 'currency_from, currency_to, exchange_rate'
        values = '"'+DEFAULT_CURRENCY+'","'+currency+'",'+exchange_rate
        insert_query = f"INSERT INTO {table_name} ({columns}) VALUES ({values}) ON DUPLICATE KEY UPDATE exchange_rate= {exchange_rate}"

        # Execute the insert query
        cursor.execute(insert_query)
    except Exception as err:
        traceback.print_exc()
# Commit the changes to the database
cnx.commit()
cursor.close()
cnx.close()