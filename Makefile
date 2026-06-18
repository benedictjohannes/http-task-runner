.PHONY: build config-generateSchema clean

build:
	go build -o http-task-runner .

config-generate-schema:
	bunx ts-json-schema-generator --path ./config.schema.ts --type ConfigSchema > ./config.schema.json

clean:
	rm -f http-task-runner
