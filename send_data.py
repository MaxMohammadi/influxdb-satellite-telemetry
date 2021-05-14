from datetime import datetime
from influxdb import InfluxDBClient

client = InfluxDBClient('localhost', 8086, 'admin', 'Password1', 'mydb')
client.create_database('mydb')
client.get_list_database()
client.switch_database('mydb')

json_payload = []
data = {}

json_payload.append(data)

client.write_points(json_payload)