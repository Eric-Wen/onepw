package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/labstack/gommon/color"
	"github.com/mkideal/cli"
	"github.com/mkideal/onepw/core"
	"github.com/mkideal/pkg/textutil"
)

func main() {
	cli.SetUsageStyle(cli.ManualStyle)
	if err := cli.Root(root,
		cli.Tree(help),
		cli.Tree(version),
		cli.Tree(initCmd),
		cli.Tree(add),
		cli.Tree(remove),
		cli.Tree(list),
	).Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//--------
// Config
//--------

type Configure interface {
	Filename() string
	MasterPassword() string
}

type Config struct {
	Master string `cli:"master" usage:"master password" dft:"$PASSWORD_MASTER"`
}

func (cfg Config) Filename() string {
	return "password.data"
}

func (cfg Config) MasterPassword() string {
	return cfg.Master
}

var box *core.Box

//--------------
// root command
//--------------

type rootT struct {
	cli.Helper
	Version bool `cli:"!v,version" usage:"display version information"`
	Config
}

var root = &cli.Command{
	Name: os.Args[0],
	Desc: textutil.Tpl("{{.onepw}} is a command line tool for managing passwords, open-source on {{.repo}}", map[string]string{
		"onepw": color.Bold("onepw"),
		"repo":  color.Blue("https://github.com/mkideal/onepw"),
	}),
	Text: textutil.Tpl(`{{.usage}}: {{.onepw}} <COMMAND> [OPTIONS]

{{.basicworkflow}}:

	#1. init, create file password.data
	$> {{.onepw}} init

	#2. add a new password
	$> {{.onepw}} add --label=email -u user@example.com
	type the password:
	repeat the password:

	#3. list all passwords
	$> {{.onepw}} list

	#optional
	# upload cloud(e.g. dropbox or github or bitbucket ...)`, map[string]string{
		"onepw":         color.Bold("onepw"),
		"usage":         color.Bold("Usage"),
		"basicworkflow": color.Bold("Basic workflow"),
	}),
	Argv: func() interface{} { return new(rootT) },

	OnBefore: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*rootT)
		if argv.Help || len(ctx.Args()) == 0 {
			ctx.WriteUsage()
			return cli.ExitError
		}
		if argv.Version {
			ctx.String("%s\n", appVersion)
			return cli.ExitError
		}
		return nil
	},

	OnRootBefore: func(ctx *cli.Context) error {
		if argv := ctx.Argv(); argv != nil {
			if t, ok := argv.(Configure); ok {
				repo := core.NewFileRepository(t.Filename())
				box = core.NewBox(repo)
				if t.MasterPassword() != "" {
					return box.Init(t.MasterPassword())
				}
				return nil
			}
		}
		return fmt.Errorf("box is nil")
	},

	Fn: func(ctx *cli.Context) error {
		return nil
	},
}

//--------------
// help command
//--------------

var help = cli.HelpCommand("display help")

//-----------------
// version command
//-----------------

const appVersion = "v0.0.1"

var version = &cli.Command{
	Name:   "version",
	Desc:   "display version",
	NoHook: true,

	Fn: func(ctx *cli.Context) error {
		ctx.String(appVersion + "\n")
		return nil
	},
}

//--------------
// init command
//--------------
type initT struct {
	cli.Helper
	Config
	NewMaster string `cli:"new-master" usage:"new master password"`
}

func (argv *initT) Validate(ctx *cli.Context) error {
	if argv.Filename() == "" {
		return fmt.Errorf("FILE is empty")
	}
	return nil
}

var initCmd = &cli.Command{
	Name: "init",
	Desc: "init password box or modify master password",
	Argv: func() interface{} { return new(initT) },

	OnBefore: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*initT)
		if argv.Help {
			ctx.WriteUsage()
			return cli.ExitError
		}
		if _, err := os.Lstat(argv.Filename()); err != nil {
			if os.IsNotExist(err) {
				if file, err := os.Create(argv.Filename()); err != nil {
					return err
				} else {
					file.Close()
				}
			}
		}
		return nil
	},

	Fn: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*initT)
		if argv.NewMaster != "" {
			return box.Init(argv.NewMaster)
		}
		return nil
	},
}

//-------------
// add command
//-------------
type addT struct {
	cli.Helper
	Config
	core.Password
	Pw  string `pw:"pw,password" usage:"the password" prompt:"type the password"`
	Cpw string `pw:"cpw,confirm-password" usage:"confirm password" prompt:"repeat the password"`
}

func (argv *addT) Validate(ctx *cli.Context) error {
	if argv.Pw != argv.Cpw {
		return fmt.Errorf("password mismatch")
	}
	return core.CheckPassword(argv.Pw)
}

var add = &cli.Command{
	Name: "add",
	Desc: "add a new password or update old password",
	Argv: func() interface{} {
		argv := new(addT)
		argv.Password = *core.NewEmptyPassword()
		return argv
	},
	CanSubRoute: false,

	OnBefore: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*addT)
		if argv.Help {
			ctx.WriteUsage()
			return cli.ExitError
		}
		return nil
	},

	Fn: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*addT)
		argv.Password.PlainPassword = argv.Pw
		id, ok, err := box.Add(&argv.Password)
		if err != nil {
			return err
		}
		if ok {
			ctx.String("password %s updated\n", id)
		} else {
			ctx.String("add password %d success\n", id)
		}
		return nil
	},
}

//--------
// remove
//--------

type removeT struct {
	cli.Helper
	Config
	Label   string `cli:"c,category" usage:"password label"`
	Account string `cli:"u,account" usage:"specify account"`
	Id      string `cli:"id" usage:"password id"`
	All     bool   `cli:"a,all" usage:"remove all found passwords" dft:"false"`
}

var remove = &cli.Command{
	Name: "remove",
	Desc: "remove passwords",
	Argv: func() interface{} { return new(removeT) },

	OnBefore: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*removeT)
		if argv.Help {
			ctx.WriteUsage()
			return cli.ExitError
		}
		return nil
	},

	Fn: func(ctx *cli.Context) error {
		var (
			argv = ctx.Argv().(*removeT)
			ids  []string
			err  error
		)
		if argv.Id != "" {
			ids, err = box.Remove(argv.Id, argv.All)
		} else if argv.Label != "" || argv.Account != "" {
			ids, err = box.RemoveByAccount(argv.Label, argv.Account, argv.All)
		} else if argv.All {
			ids, err = box.Clear()
		}

		if err != nil {
			return err
		}
		ctx.String("deleted passwords:\n")
		ctx.String(strings.Join(ids, "\n"))
		ctx.String("\n")
		return nil
	},
}

//------
// list
//------

type listT struct {
	cli.Helper
	Config
}

var list = &cli.Command{
	Name: "list",
	Desc: "list all passwords",
	Argv: func() interface{} { return new(listT) },

	OnBefore: func(ctx *cli.Context) error {
		argv := ctx.Argv().(*listT)
		if argv.Help {
			ctx.WriteUsage()
			return cli.ExitError
		}
		return nil
	},

	Fn: func(ctx *cli.Context) error {
		//argv := ctx.Argv().(*listT)
		return box.List(ctx)
	},
}
