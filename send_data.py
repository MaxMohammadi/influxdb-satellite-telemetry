from datetime import datetime

from influxdb_client import InfluxDBClient, Point, WritePrecision
from influxdb_client.client.write_api import SYNCHRONOUS

token = "wCm6Lbwn2__gUddkK_sFMYAtNnVfi3ti4cxDmRxLSIQtZeIsWcNkFwq2nj6Q8bPS7mQV3NIQqlVz2tzWVMIfmw=="
org = "Projects"
bucket = "Telemetry"

client = InfluxDBClient(url="http://localhost:8086", token=token)

write_api = client.write_api(write_options=SYNCHRONOUS)

# change time to the new unix stamps in excel
point1 = Point("TelemetryDataStream2").tag(
    "type", "coordinate").field(
        "longitude", -36.85039056).time(
            datetime.utcnow(), WritePrecision.NS)

point2 = Point("TelemetryDataStream2").tag(
    "type", "coordinate").field(
        "longitude", -35.8234361).time(
            datetime.utcnow(), WritePrecision.NS)

write_api.write(bucket, org, (point1, point2))

# use sequences to process large amounts of tuples from csv files

# sequence = ["TelemetryData,host=host1 used_percent=10001.43234543 1685580300"]
# write_api.write(bucket, org, sequence)

query = '''
from(bucket: "Telemetry")
    |> range(start: -1m)
    |> filter(fn: (r) => r["_measurement"] == "TelemetryDataStream2")
'''

result = client.query_api().query(query, org=org)

results = []
for table in result:
    for record in table.records:
        results.append((record.get_value(), record.get_field()))

print(results)