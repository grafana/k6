TODO: Intro

## New Features!

### Option: No Cookies Reset (#729)
A No Cookies Reset option has been added; it disables the cookies resetting after a VU iteration so that the cookies will be persisted across VUs iterations.

Description of feature.

**Docs**: [Title](http://k6.readme.io/docs/TODO)

## Bugs fixed!

* UI: The interactive `k6 login influxdb` command failed to write the supplied options to the config file. (#734)
* UI: Password input is now masked in `k6 login influxdb` and `k6 login cloud`. (#734)
* Config: Environment variables can now be used to modify k6's behavior in the `k6 login` subcommands. (#734)