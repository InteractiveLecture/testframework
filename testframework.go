package testframework

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

var tokens = make(map[string]string)

func SendNatsMessage(t *testing.T, channel string, payload map[string]interface{}) {
	path := "/nats-remote/" + channel
	re, err := json.Marshal(payload)
	require.Nil(t, err)
	PostUnauthorizedAndCheckStatusCode(t, path, string(re), 200)
}

func FindRawLocalById(t *testing.T, collection []interface{}, id, idField string) map[string]interface{} {
	var result map[string]interface{}
	for _, v := range collection {
		val := v.(map[string]interface{})
		if val[idField].(string) == id {
			result = val
			break
		}
	}
	require.NotNil(t, result)
	return result
}
func FindLocalById(t *testing.T, collection []map[string]interface{}, id, idField string) map[string]interface{} {
	var result map[string]interface{}
	for _, v := range collection {
		if v[idField].(string) == id {
			result = v
			break
		}
	}
	require.NotNil(t, result)
	return result
}

func RegisterNewUser(t *testing.T, authorities ...string) (string, string) {
	path := "/authentication-service/users"
	username := uuid.NewV4().String()
	userId := uuid.NewV4().String()
	user := `{
		"id" : "` + userId + `",
		"username": "` + username + `",
		"password": "` + username + `",
		"enabled": true,
		"authorities": [`
	for _, v := range authorities {
		user = user + `{"authority":"` + v + `",`
	}
	user = strings.TrimRight(user, ",")
	user = user + "}]}"
	PostUnauthorizedAndCheckStatusCode(t, path, user, 204)
	return userId, username
}

func PatchAuthorizedAndCheckStatusCode(t *testing.T, user, path, body string, expecedCode int, headers ...string) {
	headers = append(headers, "Authorization", "Bearer "+getToken(user))
	resp := PatchAuthorized(t, path, body, headers...)
	defer resp.Body.Close()
	require.Equal(t, expecedCode, resp.StatusCode)
}

func PatchAuthorized(t *testing.T, path, body string, headers ...string) *http.Response {
	return sendRequest(t, "PATCH", path, strings.NewReader(body), headers...)
}

func PostUnauthorizedAndCheckStatusCode(t *testing.T, path, body string, expecedCode int, headers ...string) {
	resp := PostUnauthorized(t, path, body, headers...)
	defer resp.Body.Close()
	require.Equal(t, expecedCode, resp.StatusCode)
}

func PostAuthorizedAndCheckStatusCode(t *testing.T, user, path, body string, expecedCode int, headers ...string) {
	resp := PostAuthorized(t, user, path, body, headers...)
	defer resp.Body.Close()
	require.Equal(t, expecedCode, resp.StatusCode)
}

func GetAuthorizedAndCheckStatusCode(t *testing.T, user, path string, expecedCode int, headers ...string) {
	resp := GetAuthorized(t, user, path, headers...)
	defer resp.Body.Close()
	require.Equal(t, expecedCode, resp.StatusCode)
}

func PostUnauthorized(t *testing.T, path, body string, headers ...string) *http.Response {
	headers = append(headers, "Content-Type", "application/json;charset=UTF-8")
	return sendRequest(t, "POST", path, strings.NewReader(body), headers...)
}

func PostAuthorized(t *testing.T, user, path, body string, headers ...string) *http.Response {
	headers = append(headers, "Authorization", "Bearer "+getToken(user), "Content-Type", "application/json;charset=UTF-8")
	return sendRequest(t, "POST", path, strings.NewReader(body), headers...)
}

func CheckUnauthorized(t *testing.T, path string) {
	resp := GetUnauthorized(t, path)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func ReadSingleJsonResult(t *testing.T, resp *http.Response) map[string]interface{} {
	defer resp.Body.Close()
	result := make(map[string]interface{})
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.Nil(t, err)
	return result
}

func ReadArrayJsonResult(t *testing.T, resp *http.Response) []map[string]interface{} {
	defer resp.Body.Close()
	result := make([]map[string]interface{}, 0)
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.Nil(t, err)
	require.NotZero(t, len(result))
	return result
}

func GetUnauthorized(t *testing.T, path string) *http.Response {
	return sendRequest(t, "GET", path, nil)
}

func GetAuthorized(t *testing.T, user, path string, headers ...string) *http.Response {
	headers = append(headers, "Authorization", "Bearer "+getToken(user))
	return sendRequest(t, "GET", path, nil, headers...)
}

func sendRequest(t *testing.T, requestType, path string, body io.Reader, headers ...string) *http.Response {
	host := getHost()
	req, err := http.NewRequest(requestType, "http://"+host+path, body)
	require.Nil(t, err)
	if len(headers)%2 != 0 {
		panic(fmt.Errorf("wrong number of header arguments!"))
	}
	for i := 0; i < len(headers); i = i + 2 {
		req.Header.Add(headers[i], headers[i+1])
	}
	client := http.Client{}
	resp, err := client.Do(req)
	require.Nil(t, err)
	return resp
}

func getToken(user string) string {
	if val, ok := tokens[user]; ok {
		return val
	}
	host := getHost()
	authString := "client_id=user-web-client&client_secret=user-web-client-secret&grant_type=password&username=" + user + "&password=" + user
	reader := strings.NewReader(authString)
	result, err := http.Post("http://"+host+"/authentication-service/oauth/token", "application/x-www-form-urlencoded", reader)
	if err != nil {
		panic(err)
	}
	defer result.Body.Close()
	if result.StatusCode != 200 {
		panic(errors.New("expected statuscode 200 from authentication-service, but got: " + result.Status))
	}
	token := make(map[string]interface{})
	err = json.NewDecoder(result.Body).Decode(&token)
	if err != nil {
		panic(err)
	}
	tokens[user] = token["access_token"].(string)
	return tokens[user]
}

func getHost() string {
	return os.Getenv("DH")
}
