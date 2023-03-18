from celery import Celery
from logger import getJSONLogger

import os

logger = getJSONLogger('email-service-handler')
app = Celery(
  'SendConfirmationEmailTask', 
  broker='redis://{}:{}/0'.format(
    os.environ.get("REDIS_HOST", "localhost"),
    os.environ.get("REDIS_PORT", "6379"),
  )
)


@app.task
def send_order_confirmation_email(email_address, confirmation):
    logger.info("rendering confirmation template...")
    try:
        logger.info(confirmation)
        logger.info("Email sent to the address:{}".format(email_address))
    except Exception as err:
        logger.error(err)
    return
