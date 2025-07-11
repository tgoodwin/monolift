import aiohttp
import asyncio
import os
import string
import random
import argparse


async def upload_follow(session, addr, user_0, user_1):
  # The new /follow endpoint expects a JSON body with user_id and follow_id
  payload = {
      'user_id': user_0,
      'follow_id': user_1
  }
  async with session.post(addr + '/follow', json=payload) as resp:
    return (resp.status, await resp.text())


async def upload_register(session, addr, user):
  # The new /register endpoint expects a JSON body.
  payload = {
      'user_id': user,
      'first_name': 'first_name_' + user,
      'last_name': 'last_name_' + user,
      'username': 'username_' + user,
      'password': 'password_' + user
  }
  async with session.post(addr + '/register', json=payload) as resp:
    return (resp.status, await resp.text())


async def upload_save(session, addr, user_id, num_users):
  # The new /save endpoint expects multipart/form-data
  data = aiohttp.FormData()

  # 1. Generate text content
  text = ''.join(random.choices(string.ascii_letters + string.digits, k=256))
  # user mentions
  for _ in range(random.randint(0, 5)):
    text += ' @username_' + str(random.randint(0, num_users))
  # urls
  for _ in range(random.randint(0, 5)):
    text += ' http://' + ''.join(random.choices(string.ascii_lowercase + string.digits, k=64))

  # 2. Add fields to form data
  data.add_field('user_id', str(user_id))
  data.add_field('text', text)

  # 3. Generate and add dummy image files
  num_images = random.randint(1, 4)
  for i in range(num_images):
      dummy_image_data = os.urandom(1024)  # 1KB of random data
      data.add_field(
          'images',
          dummy_image_data,
          filename=f'image_{i}.jpg',
          content_type='image/jpeg'
      )
  async with session.post(addr + '/save', data=data) as resp:
    return (resp.status, await resp.text())


def getNumNodes(file):
  return int(file.readline())


def getEdges(file):
  edges = []
  lines = file.readlines()
  for line in lines:
    edges.append(line.split())
  return edges


def printResults(results):
  success_count = 0
  failures = []
  for result in results:
    status, text = result
    if 200 <= status < 300:
      success_count += 1
    else:
      failures.append(result)

  if success_count > 0:
    print('Succeeded:', success_count)

  if failures:
    print(f'Failed: {len(failures)}')
    failure_groups = {}
    for status, text in failures:
      key = (status, text.strip())
      failure_groups[key] = failure_groups.get(key, 0) + 1

    for (status, text), count in sorted(failure_groups.items()):
      print(f'  - Status {status} ({count} times): {text}')


async def register(addr, nodes, limit=200):
    tasks = []
    all_results = []
    conn = aiohttp.TCPConnector(limit=limit)
    async with aiohttp.ClientSession(connector=conn) as session:
        print('Registering Users...')
        for user_id in range(nodes):
            task = asyncio.create_task(upload_register(session, addr, str(user_id)))
            tasks.append(task)
            if len(tasks) >= limit:
                results = await asyncio.gather(*tasks)
                all_results.extend(results)
                tasks = []
                print(f"Registered {len(all_results)}/{nodes} users...")
        if tasks:
            results = await asyncio.gather(*tasks)
            all_results.extend(results)
        print(f"Finished registering {len(all_results)} users.")
        printResults(all_results)


async def follow(addr, edges, limit=200):
    tasks = []
    all_results = []
    conn = aiohttp.TCPConnector(limit=limit)
    async with aiohttp.ClientSession(connector=conn) as session:
        print('Adding follows...')
        for i, edge in enumerate(edges):
            user_id, follow_id = edge[0], edge[1]
            tasks.append(asyncio.create_task(upload_follow(session, addr, user_id, follow_id)))
            tasks.append(asyncio.create_task(upload_follow(session, addr, follow_id, user_id)))
            if len(tasks) >= limit:
                results = await asyncio.gather(*tasks)
                all_results.extend(results)
                tasks = []
                print(f"Processed {i+1}/{len(edges)} edges...")
        if tasks:
            results = await asyncio.gather(*tasks)
            all_results.extend(results)
        print(f"Finished adding {len(all_results)} follows.")
        printResults(all_results)


async def compose(addr, nodes, limit=200):
    tasks = []
    all_results = []
    conn = aiohttp.TCPConnector(limit=limit)
    async with aiohttp.ClientSession(connector=conn) as session:
        print('Composing posts...')
        for user_id in range(nodes):
            for _ in range(random.randint(0, 10)):  # up to 10 posts per user, average 5
                tasks.append(asyncio.create_task(upload_save(session, addr, str(user_id), nodes)))
                if len(tasks) >= limit:
                    results = await asyncio.gather(*tasks)
                    all_results.extend(results)
                    tasks = []
                    print(f"Composed {len(all_results)} posts...")
        if tasks:
            results = await asyncio.gather(*tasks)
            all_results.extend(results)
        print(f"Finished composing {len(all_results)} posts.")
        printResults(all_results)


if __name__ == '__main__':

  parser = argparse.ArgumentParser('DeathStarBench social graph initializer.')
  parser.add_argument(
      '--graph', help='Graph name. (`socfb-Reed98`, `ego-twitter`, or `soc-twitter-follows-mun`)', default='socfb-Reed98')
  parser.add_argument(
      '--ip', help='IP address of socialNetwork NGINX web server. ', default='127.0.0.1')
  parser.add_argument(
      '--port', help='IP port of socialNetwork NGINX web server.', default=8080)
  parser.add_argument('--compose', action='store_true',
                      help='intialize with up to 20 posts per user', default=False)
  parser.add_argument('--limit', type=int, help='total number simultaneous connections', default=200)
  args = parser.parse_args()

  with open(os.path.join(os.path.dirname(__file__), 'datasets/social-graph', args.graph, f'{args.graph}.nodes'), 'r') as f:
    nodes = getNumNodes(f)
  with open(os.path.join(os.path.dirname(__file__), 'datasets/social-graph', args.graph, f'{args.graph}.edges'), 'r') as f:
    edges = getEdges(f)

  random.seed(1)   # deterministic random numbers

  addr = 'http://{}:{}'.format(args.ip, args.port)
  limit = args.limit
  loop = asyncio.new_event_loop()
  future = asyncio.ensure_future(register(addr, nodes, limit), loop=loop)
  loop.run_until_complete(future)
  future = asyncio.ensure_future(follow(addr, edges, limit), loop=loop)
  loop.run_until_complete(future)
  if args.compose:
    future = asyncio.ensure_future(compose(addr, nodes, limit), loop=loop)
    loop.run_until_complete(future)
