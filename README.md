# Forgejo (Codeberg)

[**Codeberg's**](https://codeberg.org) fork of [Forgejo](https://forgejo.org),
a self-hosted lightweight software forge.

Some changes made to [Forgejo's source code](https://codeberg.org/forgejo/forgejo) are:
- Codeberg-specific themes
- [Codeberg's custom landing page](https://codeberg.org/about)
- Codeberg's custom footer
- Various optimizations necessary for running a large instance
- Dirty hacks that we needed to deploy as soon as possible

Some of our patches may be helpful if you wish to operate a large Forgejo instance.
However, this repository as a whole contains a custom version of Forgejo
that is explicitly **not** intended for private use.

**Codeberg** and **Forgejo** work together, but, as projects, they remain separate.
For more information about **Forgejo** and **its relationship with Codeberg**,
take a look at [Forgejo's FAQ](https://forgejo.org/faq/).

## Reporting Issues

If you wish to **report an issue** for the changes that **Codeberg** has made
or are not sure where you should open an issue for a service hosted by **Codeberg**,
please create an issue on [Codeberg/Community](https://codeberg.org/Codeberg/Community).

Additionally, [**Forgejo**](https://forgejo.org) maintains its own
[Issue Tracker](https://codeberg.org/forgejo/forgejo/issues)
for bug reports and feature requests.

## Hosting

If you wish to host Forgejo on your own, please visit [forgejo.org](https://forgejo.org).

You can find some other repositories that help us operate **codeberg.org** in the
[Codeberg-Infrastructure organisation](https://codeberg.org/Codeberg-Infrastructure).

Our instance's **Forgejo configuration files** can be found here:
[Codeberg-Infrastructure/build-deploy-forgejo](https://codeberg.org/Codeberg-Infrastructure/build-deploy-forgejo)

> [!TIP]
> The configuration files include the settings required to enable the
> Codeberg-specific Forgejo themes, i.e. `codeberg-auto`, `codeberg-light`,
> `codeberg-dark` in your local Forgejo instance.

## Contributing

We follow the [upstream Forgejo tree](https://codeberg.org/forgejo/forgejo)
and most of the decisions that they make for everyone else.

If you want to send a patch to [codeberg.org](https://codeberg.org),
the best way to do that is probably sending a patch to Forgejo.

### Branding

Codeberg has a set of design-related guidelines,
which can be found [here](https://codeberg.org/Codeberg/Design).

## Reporting Vulnerabilities

See [security.txt](https://codeberg.org/.well-known/security.txt)
