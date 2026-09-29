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

variable "encoding" {
  type    = string
  default = "BINARY"
}

# null のときは範囲の端を制限しない（first: 最古, last: 最新）
variable "first_revision_id" {
  type    = string
  default = null
}

variable "last_revision_id" {
  type    = string
  default = null
}

resource "google_pubsub_schema" "user_event" {
  name       = "user-event-schema"
  type       = "PROTOCOL_BUFFER"
  definition = file("${path.module}/../proto/event/v1/user_event.proto")
}

resource "google_pubsub_topic" "user_event" {
  name = "user-event-topic"

  schema_settings {
    schema            = google_pubsub_schema.user_event.id
    encoding          = var.encoding
    first_revision_id = var.first_revision_id
    last_revision_id  = var.last_revision_id
  }
}

output "topic" {
  value = google_pubsub_topic.user_event.name
}

output "latest_schema_revision_id" {
  value = google_pubsub_schema.user_event.revision_id
}
