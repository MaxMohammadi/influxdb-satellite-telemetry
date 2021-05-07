# influxdb-satellite-telemetry
Using IninfluxDB for storage of simulated satellite telemetry

# Why Use InfluxDB for Spacecraft Telemetry
If you’re not already familiar, InfluxDB is a time-series database from InfluxData that is much better than relational databases like Postgres, and document databases like MongoDB, at collecting time-series data

Telegraf is an agent platform for collecting time-series data and sending it to InfluxDB, which then stores the data for later analysis and presentation.

# Installation and Running
1. Make a subdirectory for the docker container data volume: 
    ```mkdir ./influxdb-docker-data-volume && cd $_
    ```

2. Inside the subdirectory, run: 
    ```
    docker run --name influxdb \
        -p 8086:8086 \
        --volume $PWD:/var/lib/influxdb2 \
        influxdb:2.0.6
    ```

3. In the root directory, run: 
    ```
    docker run \
        --rm influxdb:2.0.6 \
        influxd print-config > config.yml
    ```

4. Edit the config.yml file in the root directory to locate the correct paths for the bolt path and engine path

5. Run the container in the root directory:
    ```
    docker run -p 8086:8086 \
        -v $PWD/config.yml:/etc/influxdb2/config.yml \
        influxdb:2.0.6
    ```

6. Set Up User at ```localhost:8086```

ex. Username: telemetry
    Password: telemetry
    Org Name: Projects
    Bucket Name: Telemetry
