#!/bin/sh
./docker-generate.sh
dart pub get
dart compile exe -o bootstrap main.dart