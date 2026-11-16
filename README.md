# Go ECS Games

Small games written in Go using the Entity Component System pattern, built with
[Ebitengine](https://ebitengine.org) and [Donburi](https://github.com/yohamta/donburi).

Made for the [Making Games in Go with ECS](https://threedots.tech/post/making-games-in-go-ecs/) post.

Play in the browser: https://threedotslabs.github.io/go-ecs-games/

## Games

- **pufferfish**: swim around, collect bubbles to grow, and avoid the other fish.
  Move with `WASD`, press `/` to toggle debug.
- **tiles**: click or tap tiles to knock them down.

## Patterns

[patterns/](patterns/README.md) is a catalog of ECS patterns for Ebitengine and Donburi games.
It's extracted from games I made before, so don't treat it as the definitive answer.
I made it for AI agents, to keep what they build in line with what's already familiar to me.

## Running

```sh
cd pufferfish # or tiles
go run .
```

## Credits

- Sprites: [Fish Pack](https://kenney.nl/assets/fish-pack) by [Kenney](https://kenney.nl) (CC0)
- Font: Kenney Mini Square Mono from [Kenney Fonts](https://kenney.nl/assets/kenney-fonts) by Kenney (CC0)
