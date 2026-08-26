set -e

# refer to https://github.com/rozky/blog_post_drafts/wiki/Kafka-generate-certificates

export VALIDITY=9999
export PASS=password
export TMPDIR=certtmp

mkdir -p $TMPDIR

cd $TMPDIR

# 1. Create private key and cert into a keystore
keytool -storetype JKS -keystore kafka.server.keystore.jks -alias localhost -keyalg RSA -validity $VALIDITY -genkey -storepass $PASS -keypass $PASS -dname "cn=JSON Export Local, ou=Message Converter, o=Global Relay Communications Inc., l=London, c=UK"

# 2. Check the generated cert
keytool -list -v -keystore kafka.server.keystore.jks -storepass $PASS

# 3. Create CA cert
openssl req -new -newkey rsa:4096 -x509 -keyout ca-key -out ca-cert -days $VALIDITY -subj "/C=UK/L=London/O=Global Relay Communications Inc./OU=Message Converter/CN=Localhost Root CA" -nodes

# 4. Convert the CA key to PKCS8 format
openssl pkcs8 --topk8 --inform PEM --outform PEM -passin pass:$PASS -in ca-key -out ca-key-pkcs8

# 5. Add generated CA to client and broker truststore
keytool -storetype JKS -keystore kafka.server.truststore.jks -alias CARoot -importcert -file ca-cert -keypass $PASS -storepass $PASS -noprompt
keytool -storetype JKS -keystore kafka.client.truststore.jks -alias CARoot -importcert -file ca-cert -keypass $PASS -storepass $PASS -noprompt

# 6. Export the certificate from the keystore for signing.
keytool -storetype JKS -keystore kafka.server.keystore.jks -certreq -file cert-file -alias localhost -ext san=dns:broker.local -keypass $PASS -storepass $PASS

# 7. Create a config file for the certificate
cat > server.cnf <<EOF
subjectAltName=DNS:localhost,DNS:broker.local,DNS:schema-registry.local
EOF

# 8. Sign the certificate, allow from both localhost and broker.local an any dns required.
openssl x509 -req -CA ca-cert -CAkey ca-key-pkcs8 -in cert-file -out cert-signed -days $VALIDITY -CAcreateserial -passin pass:$PASS -req -extfile server.cnf
rm server.cnf

# 9. Import both the CA cert and signed certificate into the broker keystore
keytool -storetype JKS -keystore kafka.server.keystore.jks -alias CARoot -importcert -file ca-cert -keypass $PASS -storepass $PASS -noprompt
keytool -storetype JKS -keystore kafka.server.keystore.jks -alias localhost -importcert -file cert-signed -keypass $PASS -storepass $PASS -noprompt

mv *.jks ../
cd ..

# 10. Check the final cert again. make sure subject alt names are present
keytool -list -v -keystore kafka.server.keystore.jks -storepass $PASS

mv kafka.server.truststore.jks kafka-truststore.jks
mv kafka.server.keystore.jks kafka-keystore.jks

rm -rf $TMPDIR
