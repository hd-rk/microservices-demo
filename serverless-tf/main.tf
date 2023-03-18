terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "4.51.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "4.51.0"
    }
  }
}

provider "google" {
  project = "cmpt-756-serverless"
  region  = "us-central1"
  zone    = "us-central1-a"
}

provider "google-beta" {
  project = "cmpt-756-serverless"
  region  = "us-central1"
  zone    = "us-central1-a"
}

data "google_project" "project" {}

data "google_service_account" "cloudrun-invoker" {
  account_id = "cloudrun-invoker"
}
# resource "google_compute_network" "vpc_network" {
#   name = "terraform-network"
# }
