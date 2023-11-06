package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	queryCode := request.QueryStringParameters["code"]

	if len(queryCode) <= 0 {
		response := getSignUpResponse()
		return response, nil
	}

	response, err := getAccessTokenResponse(queryCode)
	if err != nil {
		fmt.Printf("Error001: %s", err.Error())
	}

	return response, nil
}

type cognitoConfig struct {
	cognitoClientId     string
	cognitoClientSecret string
	cognitoDnsPoll      string
	awsRegion           string
	apiGatewayPrefix    string
	apiStage            string
	apiGatewayPath      string
	urlSignIn           string
	urlGetToken         string
	urlRedirect         string
	AuthorizationToken  string
}

func getCognitoConfig() cognitoConfig {

	c := new(cognitoConfig)
	c.cognitoClientId = os.Getenv("CLIENT_ID")
	c.cognitoClientSecret = os.Getenv("CLIENT_SECRET")
	c.apiGatewayPrefix = os.Getenv("API_GW_ID")
	c.awsRegion = os.Getenv("AWS_REGION")
	c.cognitoDnsPoll = "tech-challenge-grp36"
	c.apiStage = "default"
	c.apiGatewayPath = "cognito-callback"

	c.urlRedirect = "https://" + c.apiGatewayPrefix + ".execute-api." + c.awsRegion + ".amazonaws.com/" + c.apiStage + "/" + c.apiGatewayPath
	c.urlGetToken = "https://" + c.cognitoDnsPoll + ".auth." + c.awsRegion + ".amazoncognito.com/oauth2/token"

	params := url.Values{}
	params.Add("client_id", c.cognitoClientId)
	params.Add("redirect_uri", c.urlRedirect)

	c.urlSignIn = "https://" + c.cognitoDnsPoll + ".auth." + c.awsRegion + ".amazoncognito.com/oauth2/authorize?response_type=code&scope=email+openid+phone&" + params.Encode()

	c.AuthorizationToken = base64.StdEncoding.EncodeToString([]byte(c.cognitoClientId + ":" + c.cognitoClientSecret))

	return *c
}

func getAccessTokenResponse(code string) (events.APIGatewayProxyResponse, error) {

	cnf := getCognitoConfig()

	//--> Request

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", cnf.cognitoClientId)
	data.Set("code", code)
	data.Set("redirect_uri", cnf.urlRedirect)
	encodedData := data.Encode()

	request, err := http.NewRequest("POST", cnf.urlGetToken, strings.NewReader(encodedData))
	if err != nil {
		//fmt.Printf("001 Got error %s", err.Error())
		response := getErrorResponse(500, err)
		return response, err
	}

	request.Header.Add("Authorization", "Basic "+cnf.AuthorizationToken)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	//--> Response

	client := &http.Client{
		Timeout: time.Second * 30,
	}

	clientResponse, err := client.Do(request)
	if err != nil {
		//fmt.Printf("002 Got error %s", err.Error())
		response := getErrorResponse(500, err)
		return response, err
	}
	defer clientResponse.Body.Close()

	clientResponse.Header.Add("Content-Type", "application/json")

	if clientResponse.StatusCode != http.StatusOK {

		b, err := io.ReadAll(clientResponse.Body)
		if err != nil {
			response := getErrorResponse(500, err)
			return response, err
		}

		response := getErrorResponse(clientResponse.StatusCode, errors.New(string(b)))
		return response, err
	}

	type responseToken struct {
		IdToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	responseBody := &responseToken{}
	err = json.NewDecoder(clientResponse.Body).Decode(responseBody)
	if err != nil {
		//fmt.Printf("004 Got error %s", err.Error())
		response := getErrorResponse(500, err)
		return response, err
	}

	marshalBody, err := json.Marshal(responseBody)
	if err != nil {
		//fmt.Printf("004 Got error %s", err.Error())
		response := getErrorResponse(500, err)
		return response, err
	}

	resp := events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(marshalBody),
	}

	return resp, err
}

func getSignUpResponse() events.APIGatewayProxyResponse {

	cnf := getCognitoConfig()

	response := events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "{\"status\":\"ok\", \"message\":\"Please, access the link and proceed with the sign-in: [" + cnf.urlSignIn + "]\"}",
	}

	return response
}

func getErrorResponse(status int, err error) events.APIGatewayProxyResponse {

	response := events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       "{\"status\":\"error\", \"message-error\":\"" + err.Error() + "\"}",
	}

	return response
}
