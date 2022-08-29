package tokens

default allow = false

allow {
	input.claims.iss == "https://token.actions.githubusercontent.com"
	input.claims.repository_owner == "thepwagner"
	input.claims.repository_visibility != "public"
}
