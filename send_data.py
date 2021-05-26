from datetime import datetime

from influxdb_client import InfluxDBClient, Point, WritePrecision
from influxdb_client.client.write_api import SYNCHRONOUS

token = "wCm6Lbwn2__gUddkK_sFMYAtNnVfi3ti4cxDmRxLSIQtZeIsWcNkFwq2nj6Q8bPS7mQV3NIQqlVz2tzWVMIfmw=="
org = "Projects"
bucket = "Telemetry"

client = InfluxDBClient(url="http://localhost:8086", token=token)

write_api = client.write_api(write_options=SYNCHRONOUS)

# change time to the new unix stamps in excel
point = Point("Telemetry").tag(
    "host", "host1").field(
        "longitude", 10000.43234543).time(
            datetime.utcnow(), WritePrecision.NS)

write_api.write(bucket, org, point)

# sequence = ["mem,host=host1 used_percent=10000.43234543"]
# write_api.write(bucket, org, sequence)

query = '''
from(bucket: "Telemetry")
    |> range(start: -1m)
'''

print(client.query_api().query(query, org=org))