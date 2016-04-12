Annotate takes annotations in a defined schema. The intent is to support multiple backends, but currently only works with elasticsearch. 

The fields of an annotation and there suggested ussage are:

 * Id: This is a GUID. The GUID is set by the API, so you should only specify it when are changing an existing annotation
 * StartDate / EndDate: The start and end date of the annotation. When an annotation is an event without duration, StartDate equals the EndDate. The API can take times in either RFC3339 as referenced in [Go's time package](https://golang.org/pkg/time/) or a unix epoch. The two should not be mixed in the same annotation. 
 * CreationUser: Optional, The username of a human creating the annotation. For machine generated annotations, it is suggested to leave this field blank so the two can be distinguished.
 * Url: Optional, An optional link, like http://bosun.org
 * Source: Optional, The system that generated the annotation
 * Host: Optional,  hostname if the host. Recommended convention is all lowercase shortnames
 * Owner: Optional, The team or group that the annotation relates to
 * Message: Required, free form text 


#REST End Points


## Create Annotation
Annotations can be created using RFC3339 format or Epoch format (see examples below). The response will be in the same format as the annotation you created. This endpoint has the following behaviors:
 * If StartDate and EndDate are not set, the API endpoint will set the time to now
 * If StartDate or EndDate (and not both) is set, the endpoint will set the other to be the one that is set. So for single events, you need only specify one or the other
 * The Id is a GUID that is set my the API endpoint, and not the user. It gets returned in the response so you can updated the annotation in the future.
 * If an Id is set, then the annotation will be updated as long as the GUID exists. **Updates require the complete document, as all fields are overwritten (and if the field is missing, an empty value with overwrite what is there).** If the GUID is not found, an error will be returned.

**URL**

`/annotation`

**Method and Request Type**

`application/json`

`POST` | `PUT`
 
**URL Params**

*None*

**Data Params**

*Example RFC3339 Format*

```
{
  "Id": "",
  "Message": "test",
  "StartDate": "2016-02-24T22:08:41-05:00",
  "EndDate": "2016-02-24T22:08:41-05:00",
  "CreationUser": "kbrandt",
  "Url": "",
  "Source": "annotate-ui",
  "Host": "",
  "Owner": "sre",
  "Category": "test"
}
```

*Example Epoch Format*

```
{
  "Id": "",
  "Message": "test",
  "StartDate": 1456369836,
  "EndDate": 1456369836,
  "CreationUser": "kbrandt",
  "Url": "",
  "Source": "annotate-ui",
  "Host": "",
  "Owner": "sre",
  "Category": "test"
}
```

**Response**

Success:

Code: 200

*Example Epoch format*

```
{
  "Id": "78a59f94-76ed-42d2-9ed0-9c7b282c4e83",
  "Message": "test",
  "CreationUser": "kbrandt",
  "Url": "",
  "Source": "annotate-ui",
  "Host": "",
  "Owner": "sre",
  "Category": "test",
  "StartDate": 1456369836,
  "EndDate": 1456369836
}
```

Failure

Code: 500

*Example*
  
```
{
    "error": "EndDate is before StartDate"}
}
```
***

## Get Annotation

**URL**

`/annotation/:id?Epoch=1`

* `id` is the GUID of the annotation
* Example: `annotation/78a59f94-76ed-42d2-9ed0-9c7b282c4e83?Epoch=1`

**Method and Request Type**

`application/json`

`GET`
 
**URL Params**

*Optional*
 * `Epoch=1` makes it so the response has time as Epoch numbers instead of RFC3339 time.

**Response**

Success:

Code: 200

*Example Epoch format*

```
{
  "Id": "78a59f94-76ed-42d2-9ed0-9c7b282c4e83",
  "Message": "test",
  "CreationUser": "kbrandt",
  "Url": "",
  "Source": "annotate-ui",
  "Host": "",
  "Owner": "sre",
  "Category": "test",
  "StartDate": 1456369836,
  "EndDate": 1456369836
}
```

Failure:

Code: 500

*Example*
  
```
{
    "error":"elastic: Error 404 (Not Found)"
}
```

***



## Update Annotation
This has the same behavior as Create Annotation, but lets you put the id in the url. Whenever updating an annotation, the full annotation gets overwritten. So if you leave a field out, it will be zero'd out. 

**URL**

`/annotation/:id`

**Method and Request Type**

`application/json`

`PUT`
 
***


## Get Field Values
Get existing unique values for on any of the following fields:
 * `Source`
 * `Host`
 * `Category`
 * `CreationUser`


**URL**

`/annotation/values/:field`
 * field must be one of the above fields
 * example: `/annotation/values/Category`

**Method and Request Type**

`application/json`

`GET`

**Response**

Success:

Code: 200

*Example*

```
[
  "test",
  "outage"
]
```

Failure

Code: 500

*Example*
  
```
{
    "error":"invalid field Sizource"
}
```

***

## Query Annotations
Partially Documented - still highly subject to change. But if you provide a StartDate and EndDate, you will get the annotations that were active during the specified window. 

**URL**

`/annotation/query`

 * Example: `annotation/query?StartDate=2016-02-23T10:09:13-05:00&EndDate=2016-02-26T10:09:13-05:00`
 
**Method and Request Type**

`application/json`

`GET`

**Response**

Success:

Code 200

*Example*

```
[
  {
    "Id": "2c5ec8f9-3a3e-4d2e-a236-6aa42ef1077b",
    "Message": "test",
    "CreationUser": "kbrandt",
    "Url": "",
    "Source": "annotate-ui",
    "Host": "",
    "Owner": "sre",
    "Category": "outage",
    "StartDate": "2016-02-24T22:10:36-05:00",
    "EndDate": "2016-02-24T22:10:36-05:00"
  },
  {
    "Id": "af83d59b-a6bb-46a6-8d16-ba0e903eab0a",
    "Message": "test",
    "CreationUser": "kbrandt",
    "Url": "",
    "Source": "annotate-ui",
    "Host": "",
    "Owner": "sre",
    "Category": "test",
    "StartDate": "2016-02-24T22:10:36-05:00",
    "EndDate": "2016-02-24T22:10:36-05:00"
  }
]
```

Error:

Code 500

*Example*

```
{
  "error": "couldn't parse EndDate as RFC3339 or epoch: parsing time \"2016-02-a26T10:09:13-05:00\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"a26T10:09:13-05:00\" as \"02\", strconv.ParseInt: parsing \"2016-02-23T10:09:13-05:00\": invalid syntax"
}
```