# Vulcanize DB

[![Join the chat at https://gitter.im/vulcanizeio/VulcanizeDB](https://badges.gitter.im/vulcanizeio/VulcanizeDB.svg)](https://gitter.im/vulcanizeio/VulcanizeDB?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

[![Build Status](https://travis-ci.org/vulcanize/vulcanizedb.svg?branch=master)](https://travis-ci.org/vulcanize/vulcanizedb)

## About

Vulcanize DB is a set of tools that make it easier for developers to write application-specific indexes and caches for dapps built on Ethereum.

## Dependencies
 - Go 1.11+
 - Postgres 10
 - Ethereum Node
   - [Go Ethereum](https://ethereum.github.io/go-ethereum/downloads/) (1.8.18+)
   - [Parity 1.8.11+](https://github.com/paritytech/parity/releases)

## Project Setup

Using Vulcanize for the first time requires several steps be done in order to allow use of the software. The following instructions will offer a guide through the steps of the process:

1. Fetching the project
2. Installing dependencies
3. Configuring shell environment
4. Database setup
5. Configuring synced Ethereum node integration
6. Data syncing

## Installation

In order to fetch the project codebase for local use or modification, install it to your `GOPATH` via:

`go get github.com/vulcanize/vulcanizedb`

Once fetched, dependencies can be installed via `go get` or (the preferred method) at specific versions via `golang/dep`, the prototype golang pakcage manager. Installation instructions are [here](https://golang.github.io/dep/docs/installation.html).

In order to install packages with `dep`, ensure you are in the project directory now within your `GOPATH` (default location is `~/go/src/github.com/vulcanize/vulcanizedb/`) and run:

`dep ensure`

After `dep` finishes, dependencies should be installed within your `GOPATH` at the versions specified in `Gopkg.toml`.

Lastly, ensure that `GOPATH` is defined in your shell. If necessary, `GOPATH` can be set in `~/.bashrc` or `~/.bash_profile`, depending upon your system. It can be additionally helpful to add `$GOPATH/bin` to your shell's `$PATH`.

## Setting up the Database
1. Install Postgres
1. Create a superuser for yourself and make sure `psql --list` works without prompting for a password.
1. Execute `createdb vulcanize_public`
1. Execute `cd $GOPATH/src/github.com/vulcanize/vulcanizedb`
1. Run the migrations: `make migrate HOST_NAME=localhost NAME=vulcanize_public PORT=<postgres port, default 5432>`

    * See below for configuring additional environments

In some cases (such as recent Ubuntu systems), it may be necessary to overcome failures of password authentication from `localhost`. To allow access on Ubuntu, set localhost connections via hostname, ipv4, and ipv6 from `peer`/`md5` to `trust` in: `/etc/postgresql/<version>/pg_hba.conf`

(It should be noted that trusted auth should only be enabled on systems without sensitive data in them: development and local test databases.)

## Configuring Ethereum Node Integration
- To use a local Ethereum node, copy `environments/public.toml.example` to
  `environments/public.toml` and update the `ipcPath` and `levelDbPath`.
  - `ipcPath` should match the local node's IPC filepath:
      - For Geth:
        - The IPC file is called `geth.ipc`.
        - The geth IPC file path is printed to the console when you start geth.
        - The default location is:
          - Mac: `<full home path>/Library/Ethereum`
          - Linux: `<full home path>/ethereum/geth.ipc`

      - For Parity:
        - The IPC file is called `jsonrpc.ipc`.
        - The default location is:
          - Mac: `<full home path>/Library/Application\ Support/io.parity.ethereum/`
          - Linux: `<full home path>/local/share/io.parity.ethereum/`
          
  - `levelDbPath` should match Geth's chaindata directory path.
      - The geth LevelDB chaindata path is printed to the console when you start geth.
      - The default location is:
          - Mac: `<full home path>/Library/Ethereum/geth/chaindata`
          - Linux: `<full home path>/ethereum/geth/chaindata`
      - `levelDbPath` is irrelevant (and `coldImport` is currently unavailable) if only running parity.

- See `environments/infura.toml` to configure commands to run against infura, if a local node is unavailable. (Support is currently experimental, at this time.)

## Start syncing with postgres
Syncs VulcanizeDB with the configured Ethereum node.
1. Start the node
    - If node state is not yet fully synced, Vulcanize will not be able to operate on the fetched data. You will need to wait for the initial sync to finish.
1. Start the vulcanize_db sync
    - Execute `./vulcanizedb sync --config <path to config.toml>`
    - Or to sync from a specific block: `./vulcanizedb sync --config <config.toml> --starting-block-number <block-number>`

## Alternatively, sync from Geth's underlying LevelDB
Sync VulcanizeDB from the LevelDB underlying a Geth node.
1. Assure node is not running, and that it has synced to the desired block height.
1. Start vulcanize_db
   - `./vulcanizedb coldImport --config <config.toml>`
1. Optional flags:
    - `--starting-block-number <block number>`/`-s <block number>`: block number to start syncing from
    - `--ending-block-number <block number>`/`-e <block number>`: block number to sync to
    - `--all`/`-a`: sync all missing blocks

## Running the Tests

In order to run the full test suite, a test database must be prepared. By default, the rests use a database named `vulcanize_private`. Create the database in Postgres, and run migrations on the new database in preparation for executing tests:

`make migrate HOST_NAME=localhost NAME=vulcanize_private PORT=<postgres port, default 5432>`

Ginkgo is declared as a `dep` package test execution. Linting and tests can be run together via a provided `make` task:

`make test`

Tests can be run directly via Ginkgo in the project's root directory:

`ginkgo -r`

## Start full environment in docker by single command

### Geth Rinkeby

make command        | description
------------------- | ----------------
rinkeby_env_up      | start geth, postgres and rolling migrations, after migrations done starting vulcanizedb container
rinkeby_env_deploy  | build and run vulcanizedb container in rinkeby environment
rinkeby_env_migrate | build and run rinkeby env migrations
rinkeby_env_down    | stop and remove all rinkeby env containers

Success run of the VulcanizeDB container require full geth state sync,
attach to geth console and check sync state:

```bash
$ docker exec -it rinkeby_vulcanizedb_geth geth --rinkeby attach
...
> eth.syncing
false
```

If you have full rinkeby chaindata you can move it to `rinkeby_vulcanizedb_geth_data` docker volume to skip long wait of sync.
