# mountain_climber

This project is a golang web service which can return elevation data given a single or set of latitudes and longitudes.

You must have a dataset folder available with GeoTIFFs which the service can use.

Startup:
go run main.go [dataset_folder]

Examples:

curl -XGET "http://localhost:8000/v1/get_elevation?lat=[lat]&lng=[lng]"

curl -XGET "http://localhost:8000/v1/get_elevations?points=[lat1],[lng1],[lat2],[lng2]"

