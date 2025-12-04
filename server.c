#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <pthread.h>

#define PORT 8080

void error(const char *msg) {
	perror(msg);
	exit(1); 
}

void *handle_client(void *arg) {
	int client_fd = *(int*)arg;
	free(arg);
	
	char buffer[1024];
	char client_name[100];
	int valread;
		
	char *welcome = "isim giriniz:\n> ";
	send(client_fd, welcome, strlen(welcome), 0);
	
	valread = read(client_fd, buffer, sizeof(buffer));
	if (valread <= 0) {
		close(client_fd);
		pthread_exit(NULL);
	}

	buffer[valread] = '\0';
	strncpy(client_name, buffer, sizeof(client_name)-1);
	client_name[strcspn(client_name, "\n")] = 0;
	printf("Yeni istemci ismi: %s\n", client_name);

	send(client_fd, "> ", 2, 0);

	while ((valread = read(client_fd, buffer, sizeof(buffer))) > 0) {
		buffer[valread] = '\0';
		printf("[%s]: %s\n", client_name, buffer);
		send(client_fd, buffer, valread, 0);
		memset(buffer, 0, sizeof(buffer));
	}
	
	printf("%s baglantiyi kapatti.\n", client_name);
	close(client_fd);
	pthread_exit(NULL);
}

int main() {
	int server_fd;
	struct sockaddr_in address;
	int opt = 1;
	socklen_t addrlen = sizeof(address);
	
	if ((server_fd = socket(AF_INET, SOCK_STREAM, 0)) < 0)
		error("socket olusturulamadi");
	
	if (setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0)
		error("setsockopt hatasi");
		
	address.sin_family = AF_INET;
	address.sin_addr.s_addr = INADDR_ANY;
	address.sin_port = htons(PORT);
	
	if (bind(server_fd, (struct sockaddr *)&address, sizeof(address)) < 0)
		error("bind hatasi");
	
	if (listen(server_fd, 10) < 0)
		error("dinlenme hatasi");
		
	printf("sunucu baslatildi, %d portunda dinleniyor...\n", PORT);
	
	while (1) {
		int *new_socket = malloc(sizeof(int));
		if ((*new_socket = accept(server_fd, (struct sockaddr *)&address, &addrlen)) < 0) {
			perror("accept hatasi");
			free(new_socket);
			continue;
		}
		
		pthread_t tid;
		pthread_create(&tid, NULL, handle_client, new_socket);
		pthread_detach(tid);
	}
	
	close(server_fd);
	return 0;
}
