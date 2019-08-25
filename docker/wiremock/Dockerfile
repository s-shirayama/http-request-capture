FROM openjdk:alpine

RUN apk add --no-cache curl \
 && curl http://repo1.maven.org/maven2/com/github/tomakehurst/wiremock-standalone/2.23.2/wiremock-standalone-2.23.2.jar -o /etc/wiremock-standalone-2.23.2.jar

ENTRYPOINT ["java", "-jar", "/etc/wiremock-standalone-2.23.2.jar", "--preserve-host-header", "--match-headers=Host", "--verbose", "--https-port", "8081"]
CMD [""]
EXPOSE 8080 8081
