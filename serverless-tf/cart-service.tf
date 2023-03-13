resource "google_cloud_run_service" "cart" {
  name     = "demo-serverless-cart"
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
        image = "gcr.io/cmpt-756-serverful/demo/cartservice@sha256:2279862a9918c719c03221bd8d7fd0d1ea74acef7870679945ef889641cb95fb"
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
          name  = "REDIS_ADDR"
          value = "10.3.16.3:6379"
        }
      }
      container_concurrency = 100
      timeout_seconds       = 60
    }
  }
}

resource "google_cloud_run_service_iam_policy" "cart" {
  location = google_cloud_run_service.cart.location
  project  = google_cloud_run_service.cart.project
  service  = google_cloud_run_service.cart.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
