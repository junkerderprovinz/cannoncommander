#!/bin/sh
# Stands in for the server: reports who it runs as and ends, which ends the
# entrypoint too.
echo "SERVER $(id -u):$(id -g)"
