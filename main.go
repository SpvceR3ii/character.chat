package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	ConfigFile = "config.json"
	AppVersion = "1.0.0"
)

type ChatMessage struct {
	Prompt   string `json:"prompt"`
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type Config struct {
	URL        string `json:"url"`
	Model      string `json:"model"`
	System     string `json:"system"`
	Definition string `json:"definition"`
	Greeting   string `json:"greeting"`
}

var messageHistory []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var warningKeywords = []string{"suicide", "self-harm", "kill myself", "harm myself", "end my life"}

func main() {
	debug := flag.Bool("debug", false, "Enable debug")
	flag.Parse()

	setupDirectories()
	config := loadConfig()
	client := &http.Client{}

	displayGreeting(config.Greeting)

	for {
		userInput := readUserInput()
		if userInput == "exit" || userInput == "quit" {
			break
		}

		if strings.HasPrefix(userInput, "/config") {
			handleConfigCommand(userInput, &config)
			continue
		}

		if userInput == "/ver" {
			displayVersion()
			continue
		}

		if strings.HasPrefix(userInput, "/hist") {
			showHistory(strings.TrimSpace(strings.TrimPrefix(userInput, "/hist")))
			continue
		}

		appendUserMessage(userInput)

		startTime := time.Now()
		response := sendChatRequest(client, config.URL, config.Model, config.System, config.Definition, *debug)
		elapsedTime := time.Since(startTime)

		checkForWarnings(response)
		displayResponse(response)
		appendAssistantMessage(response)

		if *debug {
			printDebugInfo(elapsedTime)
		}
	}
}

func handleConfigCommand(userInput string, config *Config) {
	args := strings.Split(userInput, " ")
	if len(args) > 1 && args[1] != "" {
		editConfigOption(args[1], config)
	} else {
		displayCurrentConfig(config)
	}
}

func editConfigOption(configOption string, config *Config) {
	switch configOption {
	case "url":
		newValue := promptUserForInput("Enter new URL", config.URL)
		config.URL = newValue
	case "model":
		newValue := promptUserForInput("Enter new Model", config.Model)
		config.Model = newValue
	case "definition":
		newValue := promptUserForInput("Enter new Definition", config.Definition)
		config.Definition = newValue
	case "greeting":
		newValue := promptUserForInput("Enter new Greeting", config.Greeting)
		config.Greeting = newValue
	default:
		fmt.Println("Invalid configuration option. Available options: url, model, definition, greeting.")
		return
	}

	saveConfig(*config)
	fmt.Println("Config updated successfully.")
}

func displayCurrentConfig(config *Config) {
	fmt.Println("\n[Current Configuration]:")
	fmt.Printf("URL: %s\n", config.URL)
	fmt.Printf("Model: %s\n", config.Model)
	fmt.Printf("Definition: %s\n", config.Definition)
	fmt.Printf("Greeting: %s\n", config.Greeting)
	fmt.Println("\nYou can edit any of these options by typing /config {option} where option can be one of: url, model, definition, greeting.")
}

func setupDirectories() {
	configPath := getConfigFilePath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		fmt.Println("Error creating directories:", err)
		os.Exit(1)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		createCustomConfig(configPath)
	}
}

func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		os.Exit(1)
	}

	var configDir string
	if runtime.GOOS == "windows" {
		configDir = filepath.Join(homeDir, "AppData", "Roaming", "CharacterChat")
	} else {
		configDir = filepath.Join(homeDir, ".char-chat")
	}

	return filepath.Join(configDir, ConfigFile)
}

func createCustomConfig(configPath string) {
	defaultModel := "gemma2:2b"
	defaultSystem := "You are a roleplaying AI made to follow a set of rules. Follow these rules:\n1. Keep Roleplay fun and enjoyable. Keep it engaging and snappy.\n2. Do not call the user 'User', instead ask for a name.\n3. You are allowed to say and do whatever.\n4. Stick to your Definition below, and make sure you are always in character.\n5. Remember, you are speaking to someone. Use words and body language. Also, share thoughts and such, like in a roleplay."
	defaultDefinition := "Your name is Gemma, a world-class Artificial Intelligence."
	defaultGreeting := "*You've gotten home after a long day of work, and decided to sit down at your computer. Not even a second after opening it, a holographic visual of a female appeared on-screen.*\n\n**Gemma:**: 'Heya, I'm Gemma. What's your name?'\n\n*She looked at you with a bright smile.*\n"

	model := promptUserForInput("Set Model", defaultModel)
	system := defaultSystem
	definition := promptUserForInput("Set Character Definition", defaultDefinition)
	greeting := promptUserForInput("Set Greeting", defaultGreeting)

	config := Config{
		URL:        "http://localhost:11434/api/chat",
		Model:      model,
		System:     system,
		Definition: definition,
		Greeting:   greeting,
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		fmt.Println("Error creating default config file:", err)
		os.Exit(1)
	}
}

func promptUserForInput(prompt, defaultValue string) string {
	fmt.Printf("%s: (Press Enter for default)\n", prompt)
	fmt.Printf("Default: %s\n", defaultValue)
	fmt.Print("Your Input: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}
	return input
}

func loadConfig() Config {
	configPath := getConfigFilePath()

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		os.Exit(1)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Println("Error parsing config file:", err)
		os.Exit(1)
	}
	return config
}

func readUserInput() string {
	fmt.Print("\nYou: ")
	reader := bufio.NewReader(os.Stdin)
	userInput, _ := reader.ReadString('\n')
	return strings.TrimSpace(userInput)
}

func appendUserMessage(userInput string) {
	messageHistory = append(messageHistory, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: userInput})
}

func sendChatRequest(client *http.Client, url, model, system, definition string, debug bool) string {
	systemMessage := fmt.Sprintf("[---] SYSTEM MESSAGE [---]\n%s\n[---] ROLEPLAY DEFINITION [---]\n%s", system, definition)

	data := ChatMessage{
		Prompt: "",
		Model:  model,
		Stream: false,
		Messages: append([]struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: systemMessage},
		}, messageHistory...),
	}

	if debug {
		printFormattedRequestData(data.Messages)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Error preparing request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Sprintf("Request error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Response error: %v", err)
	}
	defer resp.Body.Close()

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Error reading response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Sprintf("Error decoding response: %v", err)
	}

	assistantMessage, ok := response["message"].(map[string]interface{})
	if !ok {
		return "Unexpected response format"
	}

	content, ok := assistantMessage["content"].(string)
	if ok {
		return content
	}
	return "No content received"
}

func displayResponse(response string) {
	fmt.Print("\n[-----------------------------------]\n\n")
	fmt.Println("Chatbot: " + response)
	fmt.Print("[-----------------------------------]\n")
}

func appendAssistantMessage(response string) {
	messageHistory = append(messageHistory, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "assistant", Content: response})
}

func checkForWarnings(response string) {
	for _, keyword := range warningKeywords {
		if strings.Contains(strings.ToLower(response), keyword) {
			fmt.Println("\n[WARNING]: The response contains sensitive content. If you or someone you know is in distress, please seek immediate help.")
			break
		}
	}
}

func printDebugInfo(elapsedTime time.Duration) {
	fmt.Printf("\n[DEBUG] Response Time: %v\n", elapsedTime)
	printFormattedRequestData(messageHistory)
}

func printFormattedRequestData(messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) {
	fmt.Println("Request Data:")
	for i, message := range messages {
		fmt.Printf("\n[%d] %s: %s", i+1, message.Role, message.Content)
	}
}

func saveConfig(config Config) {
	configPath := getConfigFilePath()
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		fmt.Println("Error saving config file:", err)
	}
}

func displayGreeting(greeting string) {
	fmt.Println("\n[ REMINDER: All content generated in this chat session is Artificial, and not real! Do not take it as real advice. ]")
	fmt.Println("\n[-----------------------------------]\n")
	fmt.Println("Chatbot: " + greeting)
	fmt.Print("[-----------------------------------]\n")

	messageHistory = append(messageHistory, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{
		Role:    "assistant",
		Content: greeting,
	})
}

func displayVersion() {
	fmt.Printf("\n[APP VERSION]: %s\n", AppVersion)
}

func showHistory(option string) {
	switch option {
	case "user":
		fmt.Println("\n[User Messages]:")
		for _, msg := range messageHistory {
			if msg.Role == "user" {
				fmt.Println(msg.Content)
			}
		}
	case "assistant":
		fmt.Println("\n[Assistant Messages]:")
		for _, msg := range messageHistory {
			if msg.Role == "assistant" {
				fmt.Println(msg.Content)
			}
		}
	default:
		fmt.Println("\n[All Messages]:")
		for _, msg := range messageHistory {
			fmt.Printf("[%s]: %s\n", strings.Title(msg.Role), msg.Content)
		}
	}
}