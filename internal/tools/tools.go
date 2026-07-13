package tools

import (
	"google.golang.org/genai"
)

func GetToolSchemas() []*genai.Tool {
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "read_file",
					Description: "Reads the contents of a file.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "The absolute or relative path to the file to read.",
							},
						},
						Required: []string{"path"},
					},
				},
				{
					Name:        "write_file",
					Description: "Creates or overwrites a file with the given content.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "The path where the file should be written.",
							},
							"content": {
								Type:        genai.TypeString,
								Description: "The content to write into the file.",
							},
						},
						Required: []string{"path", "content"},
					},
				},
				{
					Name:        "patch_file",
					Description: "Applies a search-and-replace patch to a file in the workspace.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "The absolute or relative path to the file to modify.",
							},
							"search": {
								Type:        genai.TypeString,
								Description: "The exact block of text to find in the file.",
							},
							"replace": {
								Type:        genai.TypeString,
								Description: "The block of text to replace the searched block with.",
							},
						},
						Required: []string{"path", "search", "replace"},
					},
				},
				{
					Name:        "list_directory",
					Description: "Lists the contents of a directory (files and folders).",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "The path to the directory.",
							},
						},
						Required: []string{"path"},
					},
				},
				{
					Name:        "search_files",
					Description: "Searches for files matching a query.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"query": {
								Type:        genai.TypeString,
								Description: "The filename query to search for.",
							},
						},
						Required: []string{"query"},
					},
				},
				{
					Name:        "run_bash",
					Description: "Executes a shell command in the current workspace. Restricted from using sudo, shutdown, rm -rf /, or leaving the workspace.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"command": {
								Type:        genai.TypeString,
								Description: "The bash command to execute.",
							},
						},
						Required: []string{"command"},
					},
				},
				{
					Name:        "finish",
					Description: "Signals that the task has been completed.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"message": {
								Type:        genai.TypeString,
								Description: "A brief summary of what was accomplished.",
							},
						},
						Required: []string{"message"},
					},
				},
			},
		},
	}
}
