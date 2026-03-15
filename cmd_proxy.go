package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use: "proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		// proxy 起動ごとに tmp dir を新規確定する。
		if os.Getenv("DEVSWITCH_TMPDIR") == "" {
			if _, err := initRuntimeDir(true); err != nil {
				return err
			}
		}

		if err := ensureTmpDir(); err != nil {
			return err
		}

		// 初回起動時は dynamic 設定の雛形を作成する。
		if _, err := os.Stat(dynamicPath()); err != nil {
			warnErr("write initial dynamic config", os.WriteFile(dynamicPath(), []byte(dynamicInitial), 0644))
		}

		// static 設定テンプレートへ値を埋め込む。
		conf := staticTemplate
		conf = strings.ReplaceAll(conf, "${PORT}", listenPort())
		conf = strings.ReplaceAll(conf, "${DYNAMIC_CONFIG}", dynamicPath())

		static := staticFilePath()
		warnErr("write static config", os.WriteFile(static, []byte(conf), 0644))

		c := exec.Command("traefik", "--configFile", static)

		if proxyDaemon {
			logPath := proxyLogFilePath()
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return err
			}

			c.Stdout = logFile
			c.Stderr = logFile
			c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			if err := c.Start(); err != nil {
				warnErr("close proxy log file", logFile.Close())
				return err
			}

			warnErr("write proxy pid file", os.WriteFile(proxyPIDFilePath(), []byte(strconv.Itoa(c.Process.Pid)), 0644))

			// 起動直後に死んでいないか確認して、誤検知の「started」を防ぐ。
			time.Sleep(200 * time.Millisecond)
			if !pidAlive(c.Process.Pid) {
				if err := os.Remove(proxyPIDFilePath()); err != nil && !os.IsNotExist(err) {
					warnErr("remove proxy pid file", err)
				}
				warnErr("close proxy log file", logFile.Close())
				return fmt.Errorf("proxy failed to start; see log: %s", logPath)
			}

			warnErr("close proxy log file", logFile.Close())
			fmt.Println("proxy started in daemon mode", c.Process.Pid)
			fmt.Println("proxy log", logPath)
			return nil
		}

		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}
