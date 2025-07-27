# test search stations with long enough query
GET {{host}}/search/stations?q=mannheim
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.results" exists
jsonpath "$.results" count > 0
jsonpath "$.pagination" exists
jsonpath "$.pagination.limit" == 50
jsonpath "$.pagination.offset" == 0
jsonpath "$.pagination.total" exists
jsonpath "$.pagination.total" > 0

# test search stations without authorization
GET {{host}}/search/stations?q=mannheim
HTTP 401

# test search stations with short query
GET {{host}}/search/stations?q=ma
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 400
[Asserts]
jsonpath "$.error" exists

# test search stations with no query
GET {{host}}/search/stations
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 400
[Asserts]
jsonpath "$.error" exists

# test search stations with limit to 3
GET {{host}}/search/stations?q=mann&limit=3
Content-Type: application/json
Authorization: Bearer {{api_key}}
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.results" exists
jsonpath "$.results" count == 3
jsonpath "$.pagination" exists
jsonpath "$.pagination.limit" == 3
jsonpath "$.pagination.offset" == 0
jsonpath "$.pagination.total" exists
jsonpath "$.pagination.total" > 0

