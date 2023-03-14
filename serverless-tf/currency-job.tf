resource "google_cloud_run_v2_job" "update_exchange_rates" {
  name         = "update-exchange-rates"
  location     = "us-central1"
  launch_stage = "BETA"

  template {
    task_count  = 1
    parallelism = 1
    template {
      timeout     = "60s"
      max_retries = 1
      containers {
        image = "gcr.io/cmpt-756-serverful/demo/currency-cron@sha256:64e54223e0e64943530e5c79582f4721996fcb0b02802cb5b1e4ddc8ad173bec"
        env {
          name  = "DEFAULT_CURRENCY"
          value = "USD"
        }
        env {
          name  = "CURRENCY_SUPPORT"
          value = "EUR,JPY,GBP,TRY,CAD"
        }
        env {
          name  = "REDIS_HOST"
          value = "10.3.16.3"
        }
        env {
          name  = "REDIS_PORT"
          value = "6379"
        }
        env {
          name  = "APIKEY"
          value = "DEO388ZM3UEZ34M8"
        }
        resources {
          limits = {
            memory = "512Mi"
            cpu    = "1"
          }
        }
      }
      vpc_access {
        connector = data.google_vpc_access_connector.default.id
        egress    = "ALL_TRAFFIC"
      }
    }
  }
}
