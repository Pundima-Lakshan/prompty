# Prompty: AI-Crafted Prompt Generation Tool

> Vibe Coded Use At Your Own Risk! ⚠️

Hey there! 👋 Just a heads-up right away: this entire application, **Prompty**, as event this README was **fully coded by AI**. As the "human" behind this project, my role was purely high-level guidance. My main goal was just to get this project done as quickly as possible, and the AI delivered!

---

## What is Prompty?

Prompty is a command-line interface (CLI) tool designed to help you quickly assemble prompts for Large Language Models (LLMs). It acts as your intelligent assistant, allowing you to:

- **Fuzzy Search Files:** Effortlessly find relevant files within your project using a fast, interactive fuzzy search.

- **Tag Files:** Mark specific files for inclusion in your prompt. These tagged files persist across searches, so you won't lose track of them.

- **Generate Prompts:** Combine your natural language prompt with the content of your tagged files into a structured, ready-to-use input for an LLM.

The idea is to streamline the process of providing context (like code snippets, configuration files, or documentation) to an LLM, making your interactions more efficient and effective.

---

## Features

- ⚡ **Fast Fuzzy Search:** Powered by `fzf` for quick file discovery.

- 🏷️ **Persistent Tagging:** Tagged files remain selected even after new searches, until you explicitly untag them.

- 📄 **Content Inclusion:** Automatically embeds the content of tagged files into your generated prompt.

- 📝 **Prompt Composition:** Write your main prompt text.

- 📋 **Clipboard Integration:** Easily copy the generated prompt to your clipboard.

- 💻 **Terminal-Based UI:** Built with Bubble Tea for a responsive and intuitive text-based user interface.

---

## Folder Structure

Here's a quick overview of the project's directory layout:

```
prompty/
├── internal/
│   ├── search/
│   │   └── ripgrep.go       # Handles interaction with ripgrep (rg) for file listing
│   └── ui/
│       ├── models/
│       │   ├── app.go       # The main application model, manages states (tabs)
│       │   ├── browse.go    # Model for managing and untagging selected files
│       │   ├── compose.go   # Model for user prompt input and final prompt generation
│       │   └── search.go    # Model for fuzzy searching and tagging files
│       └── styles/
│           └── styles.go    # Defines all the Lipgloss styles for the UI
└── main.go                 # Entry point of the application
```

---

## Prerequisites

Before you can run Prompty, you'll need the following installed on your system:

- **Go (1.18 or higher):** The programming language this application is built with.

  - [Download Go](https://go.dev/doc/install)

- **`fzf`:** A command-line fuzzy finder. Essential for the interactive file search.

  - [Install fzf](https://github.com/junegunn/fzf#installation)

- **`ripgrep` (or `rg`):** A fast line-oriented search tool. Used by Prompty to efficiently list files.
  - [Install ripgrep](https://github.com/BurntSushi/ripgrep#installation)

---

## Getting Started

Follow these steps to get Prompty up and running:

1. **Clone the Repository:**

   ```bash
   git clone [https://github.com/your-username/prompty.git](https://github.com/your-username/prompty.git) # Replace with your repo URL
   cd prompty
   ```

2. **Download Go Modules:**
   This command resolves and downloads all necessary Go dependencies.

   ```bash
   go mod tidy
   ```

3. **Run the Application:**
   You can run Prompty directly from its directory.

   ```bash
   go run main.go
   ```

   You should see the Prompty CLI application launch in your terminal!

---

## Usage

Once running, navigate through the tabs (Search, Browse, Compose) using `1`, `2`, `3`, `Tab`, or `Shift+Tab`.

### Search Tab (Tab 1)

- **Type to Search:** Start typing in the input box to fuzzy search for files in your current directory and its subdirectories.

- **Navigate Results:** Use `Ctrl+N` (down) and `Ctrl+P` (up) or `j`/`k` to move through the search results.

- **Tag/Untag:** Press `Ctrl+A` to tag or untag the currently selected file. Tagged files will have a `✓` next to them.

- **Clear Search:** Press `Esc` to clear your search query. If the query is empty, pressing `Esc` will show all currently tagged files.

### Browse Tab (Tab 2)

- This tab displays all the files you've tagged across your searches.

- **Navigate Files:** Use `Ctrl+N` (down) and `Ctrl+P` (up) or `j`/`k` to move through your tagged files.

- **Preview Content:** Press `Enter` on a selected file to view its content in a side panel. Press `Esc` to close the preview.

- **Untag File:** Press `Ctrl+A` to untag the currently selected file from this list.

### Compose Tab (Tab 3)

- **Your Prompt:** Enter your main request or question for the LLM in the text area.

- **Generate Prompt:** Press `Ctrl+G` to combine your text with the content of all your tagged files. The generated output will appear in a scrollable viewport.

- **Copy to Clipboard:** When viewing the generated prompt, press `Y` to copy it to your system clipboard.

- **Back to Editing:** Press `Esc` to hide the generated prompt and return to the editing area.

### Quitting the Application

- Press `Ctrl+Q` or `Ctrl+C` at any time to exit Prompty.

---

## Tested On

This application has primarily been tested on **Linux** environments. While it uses cross-platform Go libraries, `fzf` and `ripgrep` are external dependencies. Ensure they are correctly installed and configured for your operating system.

---

## Making Prompty Globally Accessible

To run `prompty` from any directory in your terminal, you can install the executable to your Go bin path:

1. **Ensure Go Bin is in your PATH:**
   Make sure your `GOPATH/bin` directory is added to your system's `PATH` environment variable. For most Go installations, this is already set up. You can check by running `echo $PATH` and looking for something like `/home/youruser/go/bin` (on Linux/macOS) or `C:\Users\youruser\go\bin` (on Windows). If it's not, you might need to add it to your shell's configuration file (e.g., `.bashrc`, `.zshrc`, `.profile`):

   ```bash
   echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc # or .zshrc/.profile
   source ~/.bashrc # or source your config file
   ```

2. **Install the Executable:**
   Navigate to the root directory of the `prompty` project (where `main.go` is located) and run:

   ```bash
   go install
   ```

   This command compiles your application and places the executable binary (named `prompty`) into your `$GOPATH/bin` directory.

3. **Run from Anywhere:**
   Now you can simply type `prompty` in your terminal from any location:

   ```bash
   prompty
   ```
