resource "google_cloud_run_service" "frontend" {
  name     = "demo-serverless-frontend"
  location = "us-central1"

  metadata {
    annotations = {
      "run.googleapis.com/client-name" = "terraform"
      "run.googleapis.com/ingress"     = "all"
      # "run.googleapis.com/ingress"     = "internal"

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
        image = "gcr.io/cmpt-756-serverful/demo/frontend@sha256:f0083d78bedd6f8aabf6899be70487d770919cffcb7c9fbc7e847702f355fd06"
        resources {
          limits = {
            memory = "128Mi"
            cpu    = "1000m"
          }
        }
        env {
          name  = "PRODUCT_CATALOG_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.product_catalog.status.0.url, "https://")}:443"
        }
        env {
          name  = "CURRENCY_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.currency.status.0.url, "https://")}:443"
        }
        env {
          name  = "CART_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.cart.status.0.url, "https://")}:443"
        }
        env {
          name  = "RECOMMENDATION_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.recommendation.status.0.url, "https://")}:443"
        }
        env {
          name  = "SHIPPING_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.shipping.status.0.url, "https://")}:443"
        }
        env {
          name  = "CHECKOUT_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.checkout.status.0.url, "https://")}:443"
        }
        env {
          name  = "AD_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.ad.status.0.url, "https://")}:443"
        }
        env {
          name  = "ENABLE_PROFILER"
          value = "0"
        }
      }
      container_concurrency = 100
      timeout_seconds       = 60
    }
  }
}

resource "google_cloud_run_service_iam_policy" "frontend" {
  location = google_cloud_run_service.frontend.location
  project  = google_cloud_run_service.frontend.project
  service  = google_cloud_run_service.frontend.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
