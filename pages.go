package main

type Pages interface {
	Fetch(page string) []byte
}

const (
	LoginPage               = "login"
	LoginSuccessPage        = "login/success"
	RegistrationPage        = "register"
	RegistrationSuccessPage = "register/success"
	LogoutPage              = "logout"
	AdminPage               = "admin"
)

type ConstPages map[string][]byte

var constPages = make(ConstPages)

func (cp ConstPages) Fetch(page string) []byte {
	if data, ok := cp[page]; ok {
		return data
	} else {
		return cp["empty"]
	}
}

func init() {
	constPages[LoginPage] = []byte(`<html>
<head>
	<title>AuthProx - Login</title>
</head>

<body>
	<h1>AuthProx Login</h1>
	<form method="POST" action="/proxy/login">
		<input type="text" name="username" placeholder="Username">
		<input type="password" name="password" placeholder="Password"> 
		<input type="submit" value="Login">
	</form>
</body>
</html>`)

	constPages[LoginSuccessPage] = []byte(`<html>
<head>
	<title>AuthProx - Login</title>
</head>

<body>
	<h1>AuthProx Login</h1>
	You have successfully logged in.
	<a href="/">Continue</a>
</body>
</html>`)

	constPages[RegistrationPage] = []byte(`<html>
<head>
	<title>AuthProx - Register</title>
	<script src='https://www.google.com/recaptcha/api.js'></script>
</head>

<body>
	<h1>AuthProx Registration</h1>
	<form method="POST" action="/proxy/register">
		<input type="text" name="username" placeholder="Username">
		<input type="password" name="password" placeholder="Password">
		<div class="g-recaptcha" data-sitekey="6LcMDgoTAAAAALJTFmdzPieTUheKAdghSG9q1_D-"></div>
		<input type="submit" value="Register">
	</form>
</body>
</html>`)

	constPages[RegistrationSuccessPage] = []byte(`<html>
<head>
	<title>AuthProx - Login</title>
</head>

<body>
	<h1>AuthProx Login</h1>
	You have successfully registered your new account.
	<a href="/proxy/login">Continue to login page</a>
</body>
</html>`)
}
