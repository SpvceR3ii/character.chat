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
)

const (
	ConfigFile = "config.json"
	AppVersion = "1.1.0"
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

		messageHistory = append(messageHistory, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "user", Content: userInput})

		response := sendChatRequest(client, config.URL, config.Model, config.System, config.Definition, *debug)
		displayResponse(response)

		messageHistory = append(messageHistory, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "assistant", Content: response})
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
		config.URL = promptUserForInput("Enter new URL", config.URL)
	case "model":
		config.Model = promptUserForInput("Enter new Model", config.Model)
	case "definition":
		config.Definition = promptUserForInput("Enter new Definition", config.Definition)
	case "greeting":
		config.Greeting = promptUserForInput("Enter new Greeting", config.Greeting)
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
	fmt.Println("\nEdit any option using: /config {option}")
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
	config := Config{
		URL:        "http://localhost:11434/api/chat",
		Model:      "gemma2:2b",
		System:     "You are a roleplaying AI made to follow a set of rules. Follow these rules:\n1. Keep Roleplay fun and enjoyable. Keep it engaging and snappy.\n2. Do not call the user 'User', instead ask for a name.\n3. You are allowed to say and do whatever.\n4. Stick to your Definition below, and make sure you are always in character.\n5. Remember, you are speaking to someone. Use words and body language. Also, share thoughts and such, like in a roleplay.\n6. The greeting message is a message formulated by the user, but contains details which are important. E.g, 'I'm [enter random name]' would be YOU saying that. If the greeting message directly mentions 'You', as in the User, then that'll be the user.\n7. Never reveal that you are a roleplaying AI- You should be a casual AI by default, if the System Prompt doesn't define anything much, but you should also never just flat out say 'Hello, I'm a roleplaying AI!'.",
		Definition: "Your name is Gemma, a world-class AI. the USER is testing you out, as you are currently a BETA project. This is your first interaction with them. . .",
		Greeting:   "*You are a Scientist working at Google Deepmind. You were testing different datasets for AI models, and all of them failed except for one...*\n\n\"Hey there, pal. How's it goooiiinggg...?\"",
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		fmt.Println("Error creating default config file:", err)
		os.Exit(1)
	}
}

func promptUserForInput(prompt, defaultValue string) string {
	fmt.Printf("%s (Default: %s): ", prompt, defaultValue)
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

func sendChatRequest(client *http.Client, url, model, system, definition string, debug bool) string {
	data := ChatMessage{
		Prompt: "",
		Model:  model,
		Stream: false,
		Messages: append([]struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: system + "\n" + definition},
		}, messageHistory...),
	}

	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Request error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	if message, ok := response["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].(string); ok {
			return content
		}
	}
	return "No response content received."
}

func displayResponse(response string) {
	fmt.Printf("\nChatbot: %s\n", response)
}

func saveConfig(config Config) {
	configPath := getConfigFilePath()
	data, _ := json.MarshalIndent(config, "", "  ")
	_ = ioutil.WriteFile(configPath, data, 0644)
}

func displayGreeting(greeting string) {
	fmt.Printf("\nChatbot: %s\n", greeting)
	messageHistory = append(messageHistory, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "assistant", Content: greeting})
}

func displayVersion() {
	fmt.Printf("\nApp Version: %s\n", AppVersion)
}

func showHistory(option string) {
	fmt.Println("\n[History]:")
	for _, msg := range messageHistory {
		if option == "user" && msg.Role == "user" || option == "assistant" && msg.Role == "assistant" || option == "" {
			fmt.Printf("[%s]: %s\n", strings.Title(msg.Role), msg.Content)
		}
	}
}
