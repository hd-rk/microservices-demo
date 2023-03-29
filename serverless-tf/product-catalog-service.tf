resource "google_cloud_run_service" "product_catalog" {
  name     = "demo-serverless-product-catalog"
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
        image = "gcr.io/google-samples/microservices-demo/productcatalogservice:v0.5.2"
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
      }
      container_concurrency = 100
      timeout_seconds       = 60
    }
  }
}

resource "google_cloud_run_service_iam_policy" "product_catalog" {
  location = google_cloud_run_service.product_catalog.location
  project  = google_cloud_run_service.product_catalog.project
  service  = google_cloud_run_service.product_catalog.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
