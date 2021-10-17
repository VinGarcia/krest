# Welcome to KRest

KRest stands for Keep it simple REST Package.

It's a very simple and powerful package wrapper over the
standard `http` package for making requests in an easier and
less verbose way.

## Sample requests

Simple requests using krest look like this:

```golang
func main() {
  // Build the client with a maximum request timeout limit of 2 seconds
  // You may specify a shorter timeout on each request using the context.
  rest := krest.New(2*time.Second)

  user, err := getUser(rest)
  if err != nil {
    log.Fatalf("unable to get user: %s", err)
  }

  err := sendUser(rest, user)
  if err != nil {
    log.Fatalf("unable to send user: %s", err)
  }
}

func getUser(rest krest.Provider) (model.User, error) {
  resp, err := rest.Get("https://example.com/user", krest.RequestData{})
  if err != nil {
    // An error is returned for any status not in range 200-299,
    // and it is safe to use the `resp` value even when there are errors.
    if resp.StatusCode == 404 {
      log.Fatalf("example.com was not found!")
    }
    // The error message contains all the information you'll need to understand
    // the error, such as Method, Request URL, response status code and even
    // the raw Payload from the error response:
    log.Fatalf("unexpected error when fetching example.com: %s", err)
  }

  // Using intermediary structs for decoding payloads like this one
  // is recomended for decoupling your internal models from the external
  // payloads:
  var parsedUser struct{
    Name string     `json:"name"`
    Age string      `json:"age"`
    Address Address `json:"address"`
  }
  err := json.Unmarshal(resp.Body, &parsedUser)
  if err != nil {
    return model.User{}, fmt.Errorf("unable to parse example user response as JSON: %s", err)
  }

  // Decode the age that was passed as string to an internal
  // format that is easier to manipulate:
  age, _ := strconv.Atoi(parsedUser.Age)

  return model.User{
    Name:    parsedUser.Name,
    Age:     age,
    Address: parsedUser.Address,
  }, nil
}

func sendUser(rest krest.Provider, user model.User) error {
  resp, err := rest.Post("https://other.example.com", krest.RequestData{
    Headers: map[string]string{
      "Authorization": "Bearer some-valid-jwt-token-goes-here",
    },

    // Using the optional retry feature:
    MaxRetries: 3,

    // Again using intermediary structs (or in this case a map) is also recommended
    // for encoding messages to match other APIs so you can keep your internal models
    // decoupled from any external dependencies:
    Body: map[string]interface{}{
      "fullname": user.Name,
      "address": user.Address,
    }
  })
  if err != nil {
    // Again this error message will already contain the info you might need to debug
    // but it is always a good idea to add more information when available:
    return fmt.Errorf("error sending user to example.com: %s", err)
  }

  return nil
}
```
