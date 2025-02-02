package main

import (
	"fmt"
	"log"
	"regexp"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `json:"username"`
}

const host = "https://localhost:3000"

func main() {

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// fmt.Println(db.Create(&User{Username: "D42"}))
	// Migrate the schema
	db.AutoMigrate(&User{})

	// db.Create(&User{Username: "D42"})

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		// return c.SendString("Hello, World!")
		var user User
		db.First(&user, 1) // find product with integer primary key
		return c.JSON(fiber.Map{
			"username": user.Username,
		})
	})

	app.Get("/users/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		c.Set("Content-Type", "application/activity+json")
		return c.JSON(fiber.Map{
			"@context": []string{
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1",
			},
			"type": "Person",
			"id":   host + "/users/" + username,
			// "name":              username,
			"preferredUsername": username,
			"discoverable":      true, // ActivityPubの仕様外だが、これがないとMisskeyに認識してもらえない？
			// "summary":           "This is a summary of " + username,
			// "icon":              "https://example.com/icon.jpg",
			"inbox":     host + "/users/" + username + "/inbox",
			"outbox":    host + "/users/" + username + "/outbox",
			"followers": host + "/users/" + username + "/followers",
			"following": host + "/users/" + username + "/following",
			"publicKey": fiber.Map{
				"id":           host + "/users/" + username + "#main-key",
				"owner":        host + "/users/" + username,
				"publicKeyPem": "-----BEGIN PUBLIC KEY...END PUBLIC KEY-----",
			},
		})
	})

	wellKnown := app.Group("/.well-known")

	wellKnown.Get("/nodeinfo", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"links": []fiber.Map{
				{
					"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
					"href": "https://localhost:3000/nodeinfo/2.1", // example.comは各自のドメインにすること
				},
			},
		})
	})

	wellKnown.Get("/nodeinfo/2.1", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"openRegistrations": false,
			"protocols":         []string{"activitypub"},
			"software": fiber.Map{
				"name":    "fub", // [a-z0-9-] のみ使用可能
				"version": "0.0.1",
			},
			"usage": fiber.Map{
				"users": fiber.Map{
					"total": 1, // 合計ユーザ数
				},
			},
			"services": fiber.Map{
				"inbound":  []string{},
				"outbound": []string{},
			},
			"metadata": fiber.Map{},
			"version":  "2.1",
		})

	})

	wellKnown.Get("/webfinger", func(c *fiber.Ctx) error {
		r, err := regexp.Compile("acct:([^@]+)@(.+)")
		if err != nil {
			log.Fatal(err)
		}

		resource := c.Query("resource")

		if resource == "" {
			return c.JSON(fiber.Map{
				"error": "resource parameter is required",
			})
		}

		matches := r.FindStringSubmatch("acct:loc@localhost:3000")
		var userName string
		if len(matches) > 1 {
			fmt.Println(matches[1]) // ユーザー名部分のみ出力
			userName = matches[1]
			// fmt.Println(matches[0]) // マッチした全体を出力
		} else {
			return c.JSON(fiber.Map{
				"error": "invalid resource parameter",
			})
		}

		c.Set("Content-Type", "application/jrd+json")
		return c.JSON(fiber.Map{
			"subject": "acct:" + resource,
			"aliases": []string{
				host + "/users/" + userName,
				host + "/@" + userName,
			},
			"links": []fiber.Map{
				{
					"rel":  "http://webfinger.net/rel/profile-page",
					"type": "text/html",
					"href": host + "/@" + userName,
				},
				{
					"rel":  "self",
					"type": "application/activity+json",
					"href": host + "/users/" + userName,
				},
				{
					"rel": "http://ostatus.org/schema/1.0/subscribe",
					// "template": "https://social.vivaldi.net/authorize_interaction?uri={uri}",
				},
			},
		})
	})

	if err := app.ListenTLS(":3000", "./localhost+2.pem", "./localhost+2-key.pem"); err != nil {
		fmt.Println("Error: ", err)
	}
}
