from celery import Celery
from logger import getJSONLogger

import os

logger = getJSONLogger('emailservice-server')
app = Celery('EmailTask', broker='redis://{}:6379/0'.format(os.environ.get("REDIS_ADDR", "localhost")))

@app.task
def send_email(email_address, content):
  logger.info("Email sent to the address:{}".format(email_address))