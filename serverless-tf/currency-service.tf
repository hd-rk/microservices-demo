resource "google_cloud_run_service" "currency" {
  name     = "demo-serverless-currency"
  location = "us-central1"

  metadata {
    annotations = {
      "run.googleapis.com/client-name" = "terraform"
      "run.googleapis.com/ingress"     = "internal"

    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  template {
    metadata {
      annotations = {
        "run.googleapis.com/vpc-access-connector" = data.google_vpc_access_connector.default.id
        "run.googleapis.com/vpc-access-egress"    = "all-traffic"
      }
    }
    spec {
      containers {
        image = "gcr.io/cmpt-756-serverful/demo/currencyservice@sha256:21999f64156a1d35ebc8b57f87f5be3c24dc3961d7e46a41b4a12bee8f1d6bad"
        resources {
          limits = {
            memory = "128Mi"
            cpu    = "1000m"
          }
        }
        ports {
          name           = "h2c"
          container_port = 8080
        }
        env {
          name  = "DISABLE_PROFILER"
          value = "1"
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
      }
      container_concurrency = 100
      timeout_seconds       = 60
    }
  }
}

resource "google_cloud_run_service_iam_policy" "currency" {
  location = google_cloud_run_service.currency.location
  project  = google_cloud_run_service.currency.project
  service  = google_cloud_run_service.currency.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
