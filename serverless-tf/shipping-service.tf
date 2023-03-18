resource "google_cloud_run_service" "shipping" {
  name     = "demo-serverless-shipping"
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
        image = "gcr.io/google-samples/microservices-demo/shippingservice:v0.5.2"
        resources {
          limits = {
            memory = "128Mi"
            cpu    = "1000m"
          }
        }
        env {
          name  = "DISABLE_PROFILER"
          value = "1"
        }
        ports {
          name           = "h2c"
          container_port = 8080
        }
      }
      container_concurrency = 100
      timeout_seconds       = 60
    }
  }
}

resource "google_cloud_run_service_iam_policy" "shipping" {
  location = google_cloud_run_service.shipping.location
  project  = google_cloud_run_service.shipping.project
  service  = google_cloud_run_service.shipping.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
