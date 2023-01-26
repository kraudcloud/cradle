#!/bin/sh
rsync -avz ./ yca7h:/opt/kraud/cradle/ --exclude /build --exclude /test
