// Magic file header to confirm directory

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	title("Starting build")

	{
		title("Confirming correct directory")
		const expectedHeader = "// Magic file header to confirm directory\n"
		buildmain, err := os.Open("build.go")
		if err != nil {
			panic(err)
		}
		foundHeader, err := bufio.NewReader(buildmain).ReadString('\n')
		if err != nil {
			panic(err)
		}

		if foundHeader != expectedHeader {
			panic("Did not find expected build.go header, are you running from the right directory?")
		}
	}

	{
		title("Formatting go files")
		runWithOutput(exec.Command("go", "fmt", "./..."))
	}

	{
		title("Removing previous build")
		err := os.RemoveAll("build")
		if err != nil {
			panic(err)
		}
	}

	{
		title("Creating build folder and copying static folder")
		err := filepath.WalkDir("static", func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel("static", path)
			if err != nil {
				return err
			}
			dst := filepath.Join("build", relPath)

			if info.IsDir() {
				return os.Mkdir(dst, fs.ModePerm)
			}
			return copyFile(path, dst)
		})

		if err != nil {
			panic(err)
		}
	}

	{
		title("Copying wasm_exec.js")
		err := copyFile(filepath.Join(runtime.GOROOT(), "misc/wasm/wasm_exec.js"), "build/wasm_exec.js")
		if err != nil {
			panic(err)
		}
	}

	{
		title("Building client")
		cmd := exec.Command("go", "build", "-o", "build/game.wasm", "./client")
		cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		runWithOutput(cmd)
	}

	{

		title("Running server")
		err := http.ListenAndServe(":8080", http.FileServer(http.Dir("build")))
		if err != nil {
			panic(err)
		}
	}
}

func title(s string) {
	fmt.Println("███", s)
}

func runWithOutput(cmd *exec.Cmd) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		title("Error running command: " + err.Error())
		os.Exit(1)
	}
}

func copyFile(src, dst string) error {
	fs, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fs.Close()
	fd, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = io.Copy(fd, fs)
	if err != nil {
		return err
	}

	return fd.Close()
}
