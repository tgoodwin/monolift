import asyncio
import aiohttp
import time
import math
import random
import string
import os
import argparse
import csv
import numpy as np


class GeometricLoadShape:
    """
    Generates a sequence of RPS levels that increase geometrically.
    """
    def __init__(self, min_rps, max_rps, num_steps):
        if min_rps <= 0:
            raise ValueError("min_rps must be positive for geometric scaling.")
        if max_rps < min_rps:
            raise ValueError("max_rps must be greater than or equal to min_rps.")
        if num_steps < 1:
            raise ValueError("num_steps must be at least 1.")

        self.rps_levels = []
        if num_steps == 1:
            self.rps_levels.append(float(min_rps))
        else:
            # Calculate the common ratio to get from min_rps to max_rps in num_steps-1 steps
            ratio = (max_rps / min_rps) ** (1 / (num_steps - 1))
            for i in range(num_steps):
                self.rps_levels.append(min_rps * (ratio ** i))

        # Ensure the last step is exactly max_rps to avoid float precision issues
        if num_steps > 1:
            self.rps_levels[-1] = float(max_rps)

    def get_steps(self):
        return self.rps_levels


def generate_compose_post_data(num_total_users):
    """
    Creates an aiohttp.FormData object for a compose post request.
    This logic is adapted from init_social_graph.py and locustfile.py.
    """
    data = aiohttp.FormData()

    # 1. Pick a random user ID
    user_id = str(random.randint(0, num_total_users - 1))

    # 2. Generate text content
    text = ''.join(random.choices(string.ascii_letters + string.digits, k=256))
    # Add random user mentions
    for _ in range(random.randint(0, 5)):
        text += ' @username_' + str(random.randint(0, num_total_users - 1))
    # Add random urls
    for _ in range(random.randint(0, 5)):
        text += ' http://' + ''.join(random.choices(string.ascii_lowercase + string.digits, k=64))

    data.add_field('user_id', user_id)
    data.add_field('text', text)

    # 3. Generate and add dummy image files
    num_images = random.randint(1, 4)
    dummy_image_data = os.urandom(1024)  # 1KB of random data
    for i in range(num_images):
        data.add_field(
            'images',
            dummy_image_data,
            filename=f'image_{i}.jpg',
            content_type='image/jpeg'
        )
    return data


async def send_request(session, url, num_total_users, results_queue):
    """Sends a single request and puts its status and latency in the results queue."""
    start_time = time.time()
    try:
        data = generate_compose_post_data(num_total_users)
        # The `timeout` here prevents a single slow request from holding a connection indefinitely.
        # The generator will continue firing new requests even if old ones are slow.
        async with session.post(url, data=data, timeout=aiohttp.ClientTimeout(total=30)) as response:
            latency = (time.time() - start_time) * 1000  # Convert to milliseconds
            # We don't need the response body, just the status.
            # This is a "fire-and-forget" action from the perspective of the main loop.
            await results_queue.put((response.status, latency))
    except Exception as e:
        # Put the exception in the queue to be counted as a failure.
        # Latency is 0 for failed requests.
        await results_queue.put((e, 0))


async def main(args):
    """The main function to orchestrate the throughput-latency analysis."""
    # A handcrafted sequence of RPS levels designed to produce a detailed curve.
    default_rps_levels = [
        20, 40, 60, 80, 100, 150, 200, 250, 300, 350, 400, 450, 500,
        600, 800, 1000, 1200, 1500, 2000, 2500, 3000
    ]
    rps_levels = default_rps_levels if args.rps_levels is None else [float(r) for r in args.rps_levels.split(',')]
    url = f"http://{args.ip}:{args.port}/save"

    print("--- Throughput-Latency Analyzer ---")
    print(f"Target URL: {url}")
    print(f"Custom RPS levels: {rps_levels}")
    print(f"Duration per level: {args.step_duration}s")
    print(f"Cool-off period: {args.cool_off}s")
    if args.output_file:
        print(f"Output CSV file: {args.output_file}")
    print("-" * 80)

    # Setup CSV writer if a file is specified
    csv_file = None
    csv_writer = None
    if args.output_file:
        csv_file = open(args.output_file, 'w', newline='')
        csv_writer = csv.writer(csv_file)
        header = [
            "Target RPS", "Actual RPS", "Avg Latency (ms)", "p95 Latency (ms)",
            "Success Count", "Failure Count"
        ]
        csv_writer.writerow(header)

    # Print console header
    print(
        "Target RPS | Actual RPS | Avg Latency (ms) | p95 Latency (ms) | Success | Fail"
    )
    print(
        "-----------+------------+------------------+------------------+---------+------"
    )

    async with aiohttp.ClientSession() as session:
        # --- Warm-up Phase ---
        # Send a few requests sequentially to warm up the service without measuring them.
        print("Warming up the service (5 requests over 5 seconds)...")
        warmup_queue = asyncio.Queue()
        for _ in range(5):
            # Awaiting send_request makes this part run sequentially.
            await send_request(session, url, args.num_users, warmup_queue)
            await asyncio.sleep(1)
        print("Warm-up complete. Starting main test.")
        print("-" * 80)
        # --- End Warm-up ---

        for i, rps in enumerate(rps_levels):
            results_queue = asyncio.Queue()
            start_time = time.time()

            # Fire requests at a constant rate for the defined step duration
            requests_fired = 0
            while time.time() - start_time < args.step_duration:
                if rps > 0:
                    delay = 1.0 / rps
                    asyncio.create_task(send_request(session, url, args.num_users, results_queue))
                    requests_fired += 1
                    await asyncio.sleep(delay)
                else:
                    await asyncio.sleep(1.0) # If rps is 0, just wait.

            # Give a grace period for in-flight requests to complete and be recorded
            await asyncio.sleep(5.0)

            # --- Process results for this step ---
            latencies = []
            success_count = 0
            failure_count = 0
            status_counts = {}

            while not results_queue.empty():
                try:
                    result, latency = results_queue.get_nowait()
                    if isinstance(result, int) and 200 <= result < 300:
                        success_count += 1
                        latencies.append(latency)
                    else:
                        failure_count += 1

                    # Track status codes/exceptions for detailed error reporting
                    status_key = type(result).__name__ if isinstance(result, Exception) else result
                    status_counts[status_key] = status_counts.get(status_key, 0) + 1

                except asyncio.QueueEmpty:
                    break

            # --- Calculate and print stats for the step ---
            actual_rps = success_count / args.step_duration if args.step_duration > 0 else 0
            avg_latency = np.mean(latencies) if latencies else 0
            p95_latency = np.percentile(latencies, 95) if latencies else 0

            # Print to console
            print(
                f"{rps:10.1f} | "
                f"{actual_rps:10.1f} | "
                f"{avg_latency:16.1f} | "
                f"{p95_latency:16.1f} | "
                f"{success_count:7d} | "
                f"{failure_count:4d}"
            )
            if failure_count > 0:
                # Print failure details for this step to provide immediate feedback
                fail_details = ", ".join([f"{k}: {v}" for k, v in status_counts.items() if not (isinstance(k, int) and 200 <= k < 300)])
                print(f"           | Failures: {fail_details}")

            # Write to CSV if a file is specified
            if csv_writer:
                row = [
                    f"{rps:.1f}", f"{actual_rps:.1f}", f"{avg_latency:.1f}",
                    f"{p95_latency:.1f}", success_count, failure_count
                ]
                csv_writer.writerow(row)

            # --- Cool-off period ---
            if args.cool_off > 0 and i < len(rps_levels) - 1:
                print(f"Cooling off for {args.cool_off} seconds...")
                await asyncio.sleep(args.cool_off)

    if csv_file:
        csv_file.close()
        print(f"\nResults saved to {args.output_file}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser('Throughput-Latency Analyzer')
    parser.add_argument("--num-users", type=int, default=962, help="Total number of users in the social graph (for generating random data)")
    parser.add_argument("--step-duration", type=int, default=30, help="Duration to apply load at each RPS level, in seconds")
    parser.add_argument("--cool-off", type=int, default=10, help="Cool-off period in seconds between load steps")
    parser.add_argument("--output-file", type=str, default=None, help="Optional path to the output CSV file")
    parser.add_argument('--ip', help='IP address of the target server.', default='127.0.0.1')
    parser.add_argument('--port', help='IP port of the target server.', default=8080)
    parser.add_argument("--rps-levels", type=str, default=None, help="Comma-separated list of RPS levels to test")
    args = parser.parse_args()

    random.seed(1)  # Deterministic random data generation
    asyncio.run(main(args))
