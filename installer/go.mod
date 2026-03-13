module github.com/lucasile/deft/installer

go 1.26.1

require (
	github.com/lucasile/deft/internal/i18n v0.0.0-00010101000000-000000000000
	github.com/manifoldco/promptui v0.9.0
	github.com/rs/zerolog v1.34.0
)

require (
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.6.1 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace github.com/lucasile/deft/internal/i18n => ../internal/i18n
