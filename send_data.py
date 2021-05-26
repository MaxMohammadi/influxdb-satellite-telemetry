from datetime import datetime

from influxdb_client import InfluxDBClient, Point, WritePrecision
from influxdb_client.client.write_api import SYNCHRONOUS

token = "wCm6Lbwn2__gUddkK_sFMYAtNnVfi3ti4cxDmRxLSIQtZeIsWcNkFwq2nj6Q8bPS7mQV3NIQqlVz2tzWVMIfmw=="
org = "Projects"
bucket = "Telemetry"

client = InfluxDBClient(url="http://localhost:8086", token=token)

write_api = client.write_api(write_options=SYNCHRONOUS)

# change time to the new unix stamps in excel
point = Point("TelemetryDataStream").tag(
    "type", "coordinate").field(
        "longitude", 123454).time(
            "2021-05-26 12:45:00 PDT")

write_api.write(bucket, org, point)

# sequence = ["TelemetryData,host=host1 used_percent=10001.43234543 1685580300"]
# write_api.write(bucket, org, sequence)

query = '''
from(bucket: "Telemetry")
    |> range(start: -1m)
    |> filter(fn:(r) => r["_measurement"] == "TelemetryDataStream")
    |> filter(fn: (r) => r["_field"] == "longitude")
'''

result = client.query_api().query(org=org, query=query)

results = []
for table in result:
    for record in table.records:
        results.append((record.get_value(), record.get_field(), record.get_measurement()))

print(results)