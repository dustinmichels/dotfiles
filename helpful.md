# Helpful

## Homebrew

```bash
# update
brew update

# see outdated
brew outdated

# upgrade all formulae
brew upgrade

# list formulas that can be cleaned up
brew cleanup -n

# remove old formulae
brew cleanup

# check things out
brew doctor

# brew leaves with tree
brew leaves | xargs brew deps --include-build --tree
```

## Yarn

```bash
# clean cache
yarn cache clean

# remove unnecessary files (careful)
yarn autoclean
```

## Docker

```bash
# clean up any resources dangling (not associated with a container)
docker system prune

# include stopped containers and unused images (not just dangling images)
docker system prune -a

# specific things
docker system prune --volumes
docker container prune
docker image prune
docker network prune
```
