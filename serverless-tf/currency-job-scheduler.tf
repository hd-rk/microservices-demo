resource "google_cloud_scheduler_job" "exchange-rate-update-scheduler" {
  provider         = google-beta
  name             = "exchange-rate-update-scheduler"
  schedule         = "*/10 * * * *"
  attempt_deadline = "60s"
  region           = google_cloud_run_v2_job.update_exchange_rates.location

  retry_config {
    retry_count = 2
  }

  http_target {
    http_method = "POST"
    uri         = "https://${google_cloud_run_v2_job.update_exchange_rates.location}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${data.google_project.project.number}/jobs/${google_cloud_run_v2_job.update_exchange_rates.name}:run"

    oauth_token {
      service_account_email = data.google_service_account.cloudrun-invoker.email
    }
  }

  # depends_on = [
  #   resource.google_project_service.cloudscheduler_api,
  #   resource.google_cloud_run_v2_job.default,
  #   resource.google_cloud_run_v2_job_iam_binding.binding
  # ]
}
