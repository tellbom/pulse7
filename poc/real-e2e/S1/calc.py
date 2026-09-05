# bill splitter: divide a total amount evenly among people
def split(total, people):
    if people < 1:
        raise ValueError("people must be >= 1")
    return total / people

def main():
    total = 120
    people = 0
    print("each pays:", split(total, people))

if __name__ == "__main__":
    main()
