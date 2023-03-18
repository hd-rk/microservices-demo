resource "google_cloud_run_v2_job" "update_recommendations" {
  name         = "update-recommendations"
  location     = "us-central1"
  launch_stage = "BETA"

  template {
    task_count  = 1
    parallelism = 1
    template {
      timeout     = "120s"
      max_retries = 1
      containers {
        image = "gcr.io/cmpt-756-serverful/demo/recommendationservice@sha256:e103100aabe0ca444cce5943cff042f1124721b9a81f841c272dfcf45bfaa8f2"
        env {
          name  = "PRODUCT_CATALOG_SERVICE_ADDR"
          value = "${trimprefix(google_cloud_run_service.product_catalog.status.0.url, "https://")}:443"
        }
        env {
          name  = "ROLE"
          value = "updater"
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
          name  = "DISABLE_PROFILER"
          value = "1"
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
