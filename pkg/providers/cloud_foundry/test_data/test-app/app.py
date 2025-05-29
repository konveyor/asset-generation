from flask import Flask
import os
import json
import mysql.connector

app = Flask(__name__)

def get_db_credentials():
    vcap_services = os.getenv("VCAP_SERVICES")
    if not vcap_services:
        raise RuntimeError("No VCAP_SERVICES found")

    services = json.loads(vcap_services)

    # Look for a service labeled 'cleardb'
    cleardb = services.get("cleardb")
    if not cleardb:
        raise RuntimeError("ClearDB service not found in VCAP_SERVICES")

    uri = cleardb[0]["credentials"]["uri"]

    # Parse URI: mysql://username:password@host:port/dbname
    import urllib.parse
    parsed = urllib.parse.urlparse(uri)

    return {
        "host": parsed.hostname,
        "port": parsed.port or 3306,
        "user": parsed.username,
        "password": parsed.password,
        "database": parsed.path.strip("/")
    }

@app.route("/")
def index():
    try:
        creds = get_db_credentials()
        conn = mysql.connector.connect(**creds)
        cursor = conn.cursor()
        cursor.execute("SELECT NOW();")
        result = cursor.fetchone()
        cursor.close()
        conn.close()
        return f"Connected to DB! Time is: {result[0]}"
    except Exception as e:
        return f"Error connecting to DB: {e}"

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    app.run(host="0.0.0.0", port=port)
