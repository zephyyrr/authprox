package main

import (
	"html/template"
	"io"
)

type Renderer interface {
	Render(w io.Writer, page Page)
}

type TemplateRenderer struct {
	*template.Template
}

var defaultRenderer = TemplateRenderer{
	Template: template.New("authprox"),
}

func init() {
	template.Must(defaultRenderer.Parse(pageTemplate))
}

func (tr TemplateRenderer) Render(w io.Writer, page Page) {
	tr.Execute(w, page)
}

type Pages interface {
	Get(name string) Page
}

const (
	MainMenuPage            = "menu"
	LoginPage               = "login"
	LoginSuccessPage        = "login/success"
	RegistrationPage        = "register"
	RegistrationSuccessPage = "register/success"
	LogoutPage              = "logout"
	AdminPage               = "admin"
)

type Page struct {
	Title   string
	Head    template.HTML
	Content template.HTML
}

type ConstPages map[string]Page

var constPages = make(ConstPages)

func (cp ConstPages) Get(name string) Page {
	if page, ok := cp[name]; ok {
		return page
	} else {
		return Page{
			Title:   "/dev/nil",
			Head:    "",
			Content: "You appear to have tried to access a page that does not exist.",
		}
	}
}

const pageTemplate = `<html>
<head>
	<title>AuthProx - {{.Title}}</title>
	{{.Head}}
</head>

<body>
	<h1>AuthProx - {{.Title}}</h1>
	{{.Content}}
</body>
</html>
`

func init() {
	constPages[LoginPage] = Page{
		Title: "Login",
		Content: `
	<form method="POST" action="/proxy/login">
		<input type="text" name="username" placeholder="Username">
		<input type="password" name="password" placeholder="Password"> 
		<input type="submit" value="Login">
	</form>`,
	}

	constPages[LoginSuccessPage] = Page{
		Title: "Login Success!",
		Content: `
	You have successfully logged in.
	<a href="/">Continue</a>`,
	}

	constPages[RegistrationPage] = Page{
		Title: "Registrations",
		Head:  "<script src='https://www.google.com/recaptcha/api.js'></script>",
		Content: `
	<form method="POST" action="/proxy/register">
		<input type="text" name="username" placeholder="Username">
		<input type="password" name="password" placeholder="Password">
		<div class="g-recaptcha" data-sitekey="6LcMDgoTAAAAALJTFmdzPieTUheKAdghSG9q1_D-"></div>
		<input type="submit" value="Register">
	</form>`,
	}

	constPages[RegistrationSuccessPage] = Page{
		Title: "Registration Successfull!",
		Content: `
	You have successfully registered your new account.
	<a href="/proxy/login">Continue to login page</a>`,
	}

	constPages[LogoutPage] = Page{
		Title: "Logout Successfull",
		Content: `
		<p>
			You have successfully been logged out.
		</p>
		<p>
			Please return at a later time!
		</p>
		`,
	}
}
