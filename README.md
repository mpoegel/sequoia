# sequoia

Sequoia is an application to create your own network of security cameras.

## Building
Sequoia uses `gocv`, which requires additional installation steps beyond the `go get`. Refer the [gocv instructions](https://gocv.io/getting-started/linux/) for more.

Generated code can be re-generated with `go generate`.

## Running

Start a camera.
```sh
./sequoia camera -s unix:///tmp/sequoia.collector
```

Start the collector.
```sh
./sequoia collect -l unix:///tmp/sequoia.collector
```

Start the web server.
```sh
./sequoia web -l :8080
```
