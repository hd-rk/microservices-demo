resource "google_cloud_run_service" "checkout" {
  name     = "demo-serverless-checkout"
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
        image = "gcr.io/cmpt-756-serverful/demo/checkoutservice@sha256:eaa4a39c233965aa57fdfe9809d65f5ad7bdb11e007da7594565ef159a46bf94"
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
          name  = "PRODUCT_CATALOG_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.product_catalog.status.0.url, "https://")}:443"
        }
        env {
          name  = "SHIPPING_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.shipping.status.0.url, "https://")}:443"
        }
        env {
          name  = "PAYMENT_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.payment.status.0.url, "https://")}:443"
        }
        env {
          name  = "EMAIL_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.email.status.0.url, "https://")}:443"
        }
        env {
          name  = "CURRENCY_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.currency.status.0.url, "https://")}:443"
        }
        env {
          name  = "CART_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.cart.status.0.url, "https://")}:443"
        }
      }
      container_concurrency = 100
      timeout_seconds       = 60
    }
  }
}

resource "google_cloud_run_service_iam_policy" "checkout" {
  location = google_cloud_run_service.checkout.location
  project  = google_cloud_run_service.checkout.project
  service  = google_cloud_run_service.checkout.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
