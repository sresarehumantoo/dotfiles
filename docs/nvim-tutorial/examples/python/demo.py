import json
import os


def total(prices: list[float]) -> float:
    result = 0.0
    for price in prices:
        result += price
    return result


def describe(name: str, count: int) -> str:
    return f"{name}: {count} items"


def main() -> None:
    prices = [1.5, 2.25, 3.0]
    print(describe("cart", total(prices)))
    print(json.dumps({"total": total(prices)}))


if __name__ == "__main__":
    main()
