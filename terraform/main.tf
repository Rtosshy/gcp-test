terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
}

variable "project_id" {
  type = string
}

resource "google_pubsub_schema" "user_event" {
  name       = "user-event-schema"
  type       = "PROTOCOL_BUFFER"
  definition = file("${path.module}/../proto/event/v1/user_event.proto")
}

resource "google_pubsub_topic" "user_event" {
  name = "user-event-topic"

  schema_settings {
    schema   = google_pubsub_schema.user_event.id
    encoding = "BINARY"
  }
}

output "topic" {
  value = google_pubsub_topic.user_event.name
}
