terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "4.51.0"
    }
  }
}

provider "google" {
  project = "cmpt-756-serverless"
  region  = "us-central1"
  zone    = "us-central1-a"
}

# resource "google_compute_network" "vpc_network" {
#   name = "terraform-network"
# }
