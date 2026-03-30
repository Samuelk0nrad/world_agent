package main

func main() {
	// env := config.NewEnv(".env", true)

	// runtime := server.NewRouter(env)
	// defer func() {
	// 	if err := runtime.Close(); err != nil {
	// 		log.Printf("error closing runtime: %v", err)
	// 	}
	// }()

	// sigCh := make(chan os.Signal, 1)
	// signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	// go func() {
	// 	<-sigCh
	// 	if err := runtime.Close(); err != nil {
	// 		log.Printf("error during shutdown: %v", err)
	// 	}
	// 	os.Exit(0)
	// }()

	// err := runtime.Router.Run(":8080")
	// if err != nil {
	// 	log.Fatalf("error occurred running the api: %v", err)
	// }
}
