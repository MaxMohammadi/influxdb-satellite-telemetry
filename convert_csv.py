import pandas as pd

df = pd.read_csv("./Telemetry_Truncated_Channels.csv")
lines = ["coordinates" 
         + ",type=TELEM"
         + " "
         + "pos_eci_x=" + str(df["pos_eci_x"][d]) + ","
         + "pos_eci_y=" + str(df["pos_eci_y"][d]) + ","
         + "pos_eci_z=" + str(df["pos_eci_z"][d]) + ","
         + "latitude=" + str(df["latitude"][d]) + ","
         + "longitude=" + str(df["longitude"][d])
         + " " + str(df["datetime"][d]) for d in range(len(df))]
thefile = open("./Telemetry_Truncated_Channels.txt", "w")
for item in lines:
    thefile.write("%s\n" % item)