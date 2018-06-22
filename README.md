# Theta Vault
Reference implementation of managed Theta wallet on video platform. In Theta stack, Vault is the primary tool for video platforms to engage their users. 

For more information on Theta technologies, please visit our website: [https://www.thetatoken.org/](https://www.thetatoken.org/)

## Build
Building vault requires [Go](https://golang.org/) (version 1.9 or later) and [Glide](https://github.com/Masterminds/glide). Once dependencies are installed, run

```
make get_vendor_deps
make install
```
This will build an executable named `vault` in `$GOPATH/bin`.

## Run
Create a `config.yml` by making a copy of `config.yml.template`. 

Vault needs to talk to a [node](https://github.com/thetatoken/theta-protocol-ledger) in Theta network. Edit the corresponding section of `config.yml` to provide details of Theta endpoint:

```
theta.chain_id: test_chain_id
theta.rpc_endpoint: http://localhost:16888/rpc
```

Vault also relies on an external SQL database to store user keys. For database schema, please refer to [reset.sql](https://github.com/thetatoken/theta-infrastructure-vault/blob/master/tools/reset.sql). 

```
db.host: localhost
db.database: test
db.table: vault
db.user: admin
db.pass: admin
```

Now simply execute `vault` and the RPC server and faucet service should start. 

## License
The vault reference implementation is licensed under the [MIT License](https://opensource.org/licenses/MIT). 
