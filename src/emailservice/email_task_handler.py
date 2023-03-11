from celery import Celery
from jinja2 import Environment, FileSystemLoader, select_autoescape, TemplateError
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

# Loads confirmation email template from file
env = Environment(
  loader=FileSystemLoader("templates"), autoescape=select_autoescape(["html", "xml"])
)
template = env.get_template("confirmation.html")

@app.task
def send_order_confirmation_email(email_address, order):
  logger.info("rendering confirmation template...")
  try:
    confirmation = template.render(order=order)
    logger.info(confirmation)
    logger.info("Email sent to the address:{}".format(email_address))
  except TemplateError as err:
    logger.error(err.message)
  return
