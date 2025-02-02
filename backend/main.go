package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
}

func generateKeyPair() (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	publicKey := &privateKey.PublicKey
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), nil
}

// const host = "https://localhost:3000"

func main() {

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// fmt.Println(db.Create(&User{Username: "D42"}))
	// Migrate the schema
	db.AutoMigrate(&User{})

	// publicKey, err := generateKeyPair()
	// if err != nil {
	// 	panic("failed to generate key pair")
	// }

	// db.Create(&User{
	// 	Username:  "D41",
	// 	PublicKey: publicKey,
	// })

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

	app.Get("/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")

		// @を含むユーザー名からローカル部分を抽出
		if parts := strings.Split(username, "@"); len(parts) > 1 {
			username = parts[1]
			fmt.Println(username)
		}

		var user User
		if err := db.First(&user, "username = ?", username); err.Error != nil {
			return c.JSON(fiber.Map{
				"error": "user not found",
			})
		}
		fmt.Println(user)
		return c.JSON(fiber.Map{
			"username": user.Username,
		})
	})

	app.Get("/users/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		var user User
		if err := db.First(&user, "username = ?", username); err.Error != nil {
			return c.JSON(fiber.Map{
				"error": "user not found",
			})
		}

		host := c.Protocol() + "://" + c.Hostname()

		c.Set("Content-Type", "application/activity+json")
		return c.JSON(fiber.Map{
			"@context": []string{
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1",
			},
			"type": "Person",
			"id":   host + "/users/" + user.Username,
			// "name":              username,
			"preferredUsername": user.Username,
			"discoverable":      true, // ActivityPubの仕様外だが、これがないとMisskeyに認識してもらえない？
			// "summary":           "This is a summary of " + username,
			// "icon":              "https://example.com/icon.jpg",
			"inbox":     host + "/users/" + user.Username + "/inbox",
			"outbox":    host + "/users/" + user.Username + "/outbox",
			"followers": host + "/users/" + user.Username + "/followers",
			"following": host + "/users/" + user.Username + "/following",
			"publicKey": fiber.Map{
				"id":           host + "/users/" + user.Username + "#main-key",
				"owner":        host + "/users/" + user.Username,
				"publicKeyPem": user.PublicKey,
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
		r, err := regexp.Compile(`^([^@]+)@(.+)$`)
		// acct:username@domain
		if err != nil {
			log.Fatal(err)
		}

		resource := c.Query("resource")

		if resource == "" {
			return c.JSON(fiber.Map{
				"error": "resource parameter is required",
			})
		}
		fmt.Println(resource)
		matches := r.FindStringSubmatch(resource)

		if len(matches) != 3 {
			return c.JSON(fiber.Map{
				"error": "invalid resource parameter format",
			})
		}

		protocol := c.Protocol()
		userName := matches[1]
		domain := protocol + "://" + matches[2]
		host := protocol + "://" + c.Hostname()
		fmt.Println(userName, domain, host)
		if domain != host {
			return c.JSON(fiber.Map{
				"error": "invalid domain",
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
