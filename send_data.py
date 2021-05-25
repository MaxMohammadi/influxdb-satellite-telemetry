from datetime import datetime

from influxdb_client import InfluxDBClient, Point, WritePrecision
from influxdb_client.client.write_api import SYNCHRONOUS

token = "wCm6Lbwn2__gUddkK_sFMYAtNnVfi3ti4cxDmRxLSIQtZeIsWcNkFwq2nj6Q8bPS7mQV3NIQqlVz2tzWVMIfmw=="
org = "Projects"
bucket = "Telemetry"

client = InfluxDBClient(url="http://localhost:8086", token=token)

write_api = client.write_api(write_options=SYNCHRONOUS)

data = "mem,host=host1 used_percent=23.43234543"
write_api.write(bucket, org, data)

point = Point("Telemetry").tag(
    "host", "host1").field(
        "used_percent", 23.43234543).time(
            datetime.utcnow(), WritePrecision.NS)

write_api.write(bucket, org, point)

sequence = ["mem,host=host1 used_percent=23.43234543",
            "mem,host=host1 available_percent=15.856523"]
write_api.write(bucket, org, sequence)

# query = f'from(bucket: \\"{bucket}\\") |> range(start: -1h)'
# tables = client.query_api().query(query, org=org)