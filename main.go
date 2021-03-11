// Copyright 2015 mokey Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	syslogHook "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/viper"
	"github.com/ubccr/mokey/server"
	"github.com/ubccr/mokey/tools"
	"github.com/urfave/cli"
)

var logFile *os.File

func init() {
	viper.SetConfigName("mokey")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/mokey/")
}

func main() {
	app := cli.NewApp()
	app.Name = "mokey"
	app.Authors = []cli.Author{cli.Author{Name: "Andrew E. Bruno", Email: "aebruno2@buffalo.edu"}}
	app.Usage = "mokey"
	app.Version = "0.5.4"
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "conf,c", Usage: "Path to conf file"},
		&cli.BoolFlag{Name: "debug,d", Usage: "Print debug messages"},
	}
	app.Before = func(c *cli.Context) error {
		conf := c.GlobalString("conf")
		if len(conf) > 0 {
			viper.SetConfigFile(conf)
		}

		err := viper.ReadInConfig()
		if err != nil {
			return fmt.Errorf("Failed reading config file - %s", err)
		}

		if c.GlobalBool("debug") {
			log.SetLevel(log.DebugLevel)
		} else {
			switch viper.GetString("log_level") {
			case "error":
				log.SetLevel(log.ErrorLevel)
			case "warn":
				log.SetLevel(log.WarnLevel)
			case "info":
				log.SetLevel(log.InfoLevel)
			case "debug":
				log.SetLevel(log.DebugLevel)
			default:
				log.SetLevel(log.WarnLevel)
			}
		}

		switch viper.GetString("log_target") {
		case "stderr":
			log.SetOutput(os.Stderr)

		case "stdout":
			log.SetOutput(os.Stdout)

		case "file":
			if len(viper.GetString("log_file")) == 0 {
				return errors.New("Please specify a log file")
			}

			logFile, err = os.OpenFile(viper.GetString("log_file"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)

			if err == nil {
				log.SetOutput(logFile)
			} else {
				return errors.New("Failed to open log file")
			}

			// reload log file when receiving SIGHUP
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, syscall.SIGHUP)

				for {
					_ = <-c
					var err error
					_ = logFile.Close()
					logFile, err = os.OpenFile(viper.GetString("log_file"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)

					if err != nil {
						panic("Failed to reload log file")
					} else {
						log.SetOutput(logFile)
						log.Info("Log file successfully reloaded")
					}
				}
			}()

		case "syslog":
			hook, err := syslogHook.NewSyslogHook("", "", syslog.LOG_INFO, "")

			if err != nil {
				return errors.New("Failed to setup syslog output")
			}
			log.AddHook(hook)

		default:
			log.SetOutput(os.Stderr)
		}

		if viper.GetString("log_format") == "json" {
			log.SetFormatter(&log.JSONFormatter{})
		} else {
			// Text formatter is created by default
		}

		// logging now setup properly

		if !viper.IsSet("enc_key") || !viper.IsSet("auth_key") {
			log.Fatal("Please ensure authentication and encryption keys are set")
		}

		return nil
	}
	app.After = func(c *cli.Context) error {
		if logFile != nil {
			return logFile.Close()
		}
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "server",
			Usage: "Run http server",
			Action: func(c *cli.Context) error {
				err := server.Run()
				if err != nil {
					log.Fatal(err)
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
		{
			Name:  "resetpw",
			Usage: "Send reset password email",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "uid, u", Usage: "User id"},
			},
			Action: func(c *cli.Context) error {
				uid := c.String("uid")
				if len(uid) == 0 {
					return cli.NewExitError(errors.New("Please provide a uid"), 1)
				}

				err := tools.SendResetPasswordEmail(uid)
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
		{
			Name:  "verify-email",
			Usage: "Re-send verify email",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "uid, u", Usage: "User id"},
			},
			Action: func(c *cli.Context) error {
				uid := c.String("uid")
				if len(uid) == 0 {
					return cli.NewExitError(errors.New("Please provide a uid"), 1)
				}

				err := tools.SendVerifyEmail(uid)
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
		{
			Name:  "status",
			Usage: "Display token status for user",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "uid, u", Usage: "User id"},
			},
			Action: func(c *cli.Context) error {
				uid := c.String("uid")
				if len(uid) == 0 {
					return cli.NewExitError(errors.New("Please provide a uid"), 1)
				}

				err := tools.Status(uid)
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		}}

	app.RunAndExitOnError()
}
