variable "VERSION" {
    default = "dev"
}

variable "PUSH_ENABLED" {
    default = false
}

group "default" {
    targets = ["ce"]
}

target "ce" {
    dockerfile = "Dockerfile"
    tags = ["jumpserver/koko:${VERSION}-ce"]
    output = PUSH_ENABLED ? ["type=registry"] : ["type=docker"]
}