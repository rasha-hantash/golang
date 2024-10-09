package config


import ( 
	"encoding/json"
	"fmt"
	"os"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

type Config struct {
	Environment          string `json:"ENVIRONMENT"`
	Auth0Domain          string `json:"AUTH0_DOMAIN"`
	OperatorClientID   string `json:"DISPATCHER_AUTH0_CLIENT_ID"`
	OperatorClientSecret string `json:"DISPATCHER_AUTH0_CLIENT_SECRET"`
	RabbitMQHost         string `json:"RABBITMQ_HOST"`
	// Add any other configuration fields you need
}

func LoadConfig() (*Config, error) {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "local"
	}

	secretName := fmt.Sprintf("%s-operator-config", env)
	region := "us-east-1" // Replace with your AWS region

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	svc := secretsmanager.New(sess)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret value: %v", err)
	}

	var secretString string
	if result.SecretString != nil {
		secretString = *result.SecretString
	} else {
		return nil, fmt.Errorf("secret value is not a string")
	}

	var config Config
	err = json.Unmarshal([]byte(secretString), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret value: %v", err)
	}

	config.Environment = env

	return &config, nil
}
