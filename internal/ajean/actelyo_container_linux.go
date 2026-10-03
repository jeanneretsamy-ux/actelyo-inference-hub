//go:build linux

package ajean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// A container has no systemd. The web process owns and reaps its engine child.
var containerEngine struct {
	sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

func actelyoContainer() bool { return os.Getenv("ACTELYO_CONTAINER") == "1" }

func containerEngineAction(action string) error {
	if action == "restart" {
		if err := containerEngineAction("stop"); err != nil {
			return err
		}
		return containerEngineAction("start")
	}
	containerEngine.Lock()
	defer containerEngine.Unlock()
	switch action {
	case "start":
		if containerEngine.cmd != nil {
			return nil
		}
		cfg := ReadConfig()
		if cfg["BIN"] == "" || cfg["MODEL"] == "" {
			return fmt.Errorf("configurez le moteur et un modèle GGUF avant de démarrer")
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		log, err := os.OpenFile(containerEngineLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		cmd := exec.Command(exe, "serve")
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			log.Close()
			return err
		}
		done := make(chan struct{})
		containerEngine.cmd, containerEngine.done = cmd, done
		go func() {
			_ = cmd.Wait()
			_ = log.Close()
			containerEngine.Lock()
			if containerEngine.cmd == cmd {
				containerEngine.cmd = nil
			}
			containerEngine.Unlock()
			close(done)
		}()
		fmt.Println("Actelyo Legal Inference : chargement du moteur en cours")
		return nil
	case "stop":
		if containerEngine.cmd == nil {
			return nil
		}
		cmd, done := containerEngine.cmd, containerEngine.done
		err := cmd.Process.Kill()
		containerEngine.Unlock()
		<-done
		containerEngine.Lock()
		if err != nil && err != os.ErrProcessDone {
			return err
		}
		return nil
	case "status":
		fmt.Printf("Actelyo Legal Inference : processus actif=%v\n", containerEngine.cmd != nil)
		return nil
	default:
		return fmt.Errorf("action %s indisponible dans le conteneur ; utilisez le redémarrage Docker", action)
	}
}

func containerEngineLogPath() string {
	return filepath.Join(os.Getenv("AJEAN_HOME"), "actelyo-engine.log")
}

func containerEngineActive() bool {
	containerEngine.Lock()
	defer containerEngine.Unlock()
	return containerEngine.cmd != nil
}

func containerEngineLogTail() string {
	data, err := os.ReadFile(containerEngineLogPath())
	if os.IsNotExist(err) {
		return "Le moteur n'a pas encore démarré."
	}
	if err != nil {
		return err.Error()
	}
	if len(data) > 32768 {
		data = data[len(data)-32768:]
	}
	return string(data)
}
