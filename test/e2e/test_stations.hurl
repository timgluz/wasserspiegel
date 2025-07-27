# test get list of stations
GET {{host}}/stations
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.stations" exists

# test unauthorized access
GET {{host}}/stations
HTTP 401

# test limit and offset parameters
GET {{host}}/stations?limit=5&offset=10
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.stations" exists
jsonpath "$.stations" count == 5
jsonpath "$.pagination" exists
jsonpath "$.pagination.limit" == 5
jsonpath "$.pagination.offset" == 10
jsonpath "$.pagination.total" exists
jsonpath "$.pagination.total" > 0

# test get a specific station
GET {{host}}/stations/rhein-mannheim
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.station" exists
jsonpath "$.station.id" == "rhein-mannheim"
jsonpath "$.station.location" exists
jsonpath "$.water_level" exists

# test unauthorized access to a specific station
GET {{host}}/stations/rhein-mannheim
HTTP 401

# test get a non-existing station
GET {{host}}/stations/non-existing-station
Authorization: Bearer {{api_key}}
HTTP 404


